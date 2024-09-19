package main

import (
	"context"
	"expvar"
	"fmt"
	"io"
	stdLog "log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anacrolix/bargle"
	"github.com/anacrolix/envpprof"
	"github.com/anacrolix/log"
	"github.com/anacrolix/tagflag"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/iplist"
	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
	"github.com/anacrolix/torrent/storage"
	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/time/rate"
)

func mainErr(ctx context.Context) error {
	// Set up logging.
	defer stdLog.SetFlags(stdLog.Flags() | stdLog.Lshortfile)
	// Set up tracing.
	tracingExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return fmt.Errorf("creating tracing exporter: %w", err)
	}
	// Set up the tracer provider.
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(tracingExporter))
	// 关闭 tracerProvider
	defer shutdownTracerProvider(ctx, tracerProvider)
	// Set the global logger.
	fmt.Println("Hello, world!")
	otel.SetTracerProvider(tracerProvider)

	main := bargle.Main{}
	// stop the profiler
	main.Defer(envpprof.Stop)
	// stop the tracer provider
	main.Defer(func() { shutdownTracerProvider(ctx, tracerProvider) })

	debug := false
	debugFlag := bargle.NewFlag(&debug)
	debugFlag.AddLong("debug")
	main.Options = append(main.Options, debugFlag.Make())
	main.Positionals = append(main.Positionals, bargle.Subcommand{Name: "download", Command: func() bargle.Command {
		dlc := DownloadCmd{}
		cmd := bargle.FromStruct(&dlc)
		cmd.DefaultAction = func() error {
			return downloadErr(downloadFlags{Debug: debug, DownloadCmd: dlc})
		}
		return cmd
	}()})
	main.Run()

	return nil
}

// Shutdown the tracer provider.
func shutdownTracerProvider(ctx context.Context, tp *trace.TracerProvider) {
	started := time.Now()
	err := tp.Shutdown(ctx)
	elapsed := time.Since(started)
	log.Levelf(log.Error, "shutting down tracer provider (took %v): %v", elapsed, err)
}

type DownloadCmd struct {
	SaveMetainfos      bool
	Mmap               bool           `help:"memory-map torrent data"`
	Seed               bool           `help:"seed after download is complete"`
	Addr               string         `help:"network listen addr"`
	MaxUnverifiedBytes *tagflag.Bytes `help:"maximum number bytes to have pending verification"`
	UploadRate         *tagflag.Bytes `help:"max piece bytes to send per second"`
	DownloadRate       *tagflag.Bytes `help:"max bytes per second down from peers"`
	PackedBlocklist    string
	PublicIP           net.IP
	Progress           bool `default:"true"`
	PieceStates        bool `help:"Output piece state runs at progress intervals."`
	Quiet              bool `help:"discard client logging"`
	Stats              bool `help:"print stats at termination"`
	Dht                bool `default:"true"`
	PortForward        bool `default:"true"`

	TcpPeers        bool `default:"true"`
	UtpPeers        bool `default:"true"`
	Webtorrent      bool `default:"true"`
	DisableWebseeds bool
	// Don't progress past handshake for peer connections where the peer doesn't offer the fast
	// extension.
	RequireFastExtension bool

	Ipv4 bool `default:"true"`
	Ipv6 bool `default:"true"`
	Pex  bool `default:"true"`

	LinearDiscard bool     `help:"Read and discard selected regions from start to finish. Useful for testing simultaneous Reader and static file prioritization."`
	TestPeer      []string `help:"addresses of some starting peers"`

	File    []string
	Torrent []string `arity:"+" help:"torrent file path or magnet uri" arg:"positional"`
}

type downloadFlags struct {
	Debug bool
	DownloadCmd
}

func downloadErr(flags downloadFlags) error {
	fmt.Println("Downloading...")
	cliConfig := torrent.NewDefaultClientConfig()
	cliConfig.DisableWebseeds = flags.DisableWebseeds
	cliConfig.DisableTCP = !flags.TcpPeers
	cliConfig.DisableUTP = !flags.UtpPeers
	cliConfig.DisableIPv4 = !flags.Ipv4
	cliConfig.DisableIPv6 = !flags.Ipv6
	cliConfig.DisableAcceptRateLimiting = true
	cliConfig.NoDHT = !flags.Dht
	cliConfig.Debug = flags.Debug
	cliConfig.Seed = flags.Seed
	cliConfig.PublicIp4 = flags.PublicIP.To4()
	cliConfig.PublicIp6 = flags.PublicIP
	cliConfig.DisablePEX = !flags.Pex
	cliConfig.DisableWebtorrent = !flags.PortForward
	cliConfig.NoDefaultPortForwarding = !flags.PortForward

	//
	if flags.PackedBlocklist != "" {
		blocklist, err := iplist.MMapPackedFile(flags.PackedBlocklist)
		if err != nil {
			return fmt.Errorf("loading packed blocklist: %v", err)
		}
		defer blocklist.Close()
		cliConfig.IPBlocklist = blocklist
	}

	// 设置监听地址
	if flags.Addr != "" {
		cliConfig.SetListenAddr(flags.Addr)
	}

	// 设置上传速率限制
	if flags.UploadRate != nil {
		// 256KB/s is the default upload rate limit.
		cliConfig.UploadRateLimiter = rate.NewLimiter(rate.Limit(*flags.UploadRate), 256<<10)
	}

	// 设置下载速率限制
	if flags.DownloadRate != nil {
		cliConfig.DownloadRateLimiter = rate.NewLimiter(rate.Limit(*flags.DownloadRate), 1<<16)
	}

	// set up the logger
	{
		logger := log.Default.WithNames("main", "client")
		if flags.Quiet {
			logger = logger.WithFilterLevel(log.Critical)
		}
		cliConfig.Logger = logger
	}

	// 是否开启扩展功能
	if flags.RequireFastExtension {
		cliConfig.MinPeerExtensions.SetBit(pp.ExtensionBitFast, true)
	}

	// 设置最大未验证的字节数
	if flags.MaxUnverifiedBytes != nil {
		cliConfig.MaxUnverifiedBytes = flags.MaxUnverifiedBytes.Int64()
	}

	// 程序接收到中段或终止信号
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 启动客户端
	client, err := torrent.NewClient(cliConfig)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	// 开启http服务
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		client.WriteStatus(w)
	})

	wg := sync.WaitGroup{}
	fataErr := make(chan error, 1)
	err = addTorrents(ctx, client, flags, &wg, func(err error) {
		select {
		case fataErr <- err:
		default:
			panic(err)
		}
	})
	if err != nil {
		return fmt.Errorf("adding torrents: %w", err)
	}

	started := time.Now()
	defer optputStats(client, flags)

	// 这段代码的关键功能是在等待所有下载操作完成的同时，
	// 处理可能发生的错误和上下文的取消。它通过使用 sync.WaitGroup 来管理并发任务，
	// 通过两个通道来同步状态，确保一旦所有任务完成或遇到错误，能够及时作出相应的处理。
	wgWaited := make(chan struct{})
	go func() {
		defer close(wgWaited)
		wg.Wait()
	}()

	select {
	case <-wgWaited:
		if ctx.Err() != nil {
			log.Print("downloaded All the torrents")
		} else {
			err = ctx.Err()
		}
	case err = <-fataErr:
	}

	clientConnStats := client.ConnStats()
	log.Printf("average download rate %s/s", humanize.Bytes(uint64(float64(clientConnStats.BytesReadUsefulData.Int64()/int64(time.Since(started).Seconds())))))

	if flags.Seed {
		if len(client.Torrents()) == 0 {
			log.Print("no torrent to seed")
		} else {
			optputStats(client, flags)
			<-ctx.Done()
		}

	}

	fmt.Printf("chunks received :%v\n", &torrent.ChunksReceived)
	spew.Dump(client.ConnStats())
	clStats := client.ConnStats()
	sentOverhead := clStats.BytesWritten.Int64() - clStats.BytesReadUsefulData.Int64()
	log.Printf(" client read %v , %.1f%% was useful data . sent %v non-data bytes ",
		humanize.Bytes(uint64(clStats.BytesRead.Int64())),
		100*float64(clStats.BytesReadUsefulData.Int64())/float64(clStats.BytesRead.Int64()),
		humanize.Bytes(uint64(sentOverhead)),
	)
	return err
}

func addTorrents(ctx context.Context, cli *torrent.Client, flags downloadFlags, wg *sync.WaitGroup, fataErr func(err error)) error {
	// 装载节点信息
	testPeers := resolveTestPeers(flags.TestPeer)
	// 遍历 args
	for _, arg := range flags.Torrent {
		t, err := func() (*torrent.Torrent, error) {
			if strings.HasPrefix(arg, "magnet:") {
				t, err := cli.AddMagnet(arg)
				if err != nil {
					return nil, fmt.Errorf("error adding magnet: %w", err)
				}
				return t, nil
			} else {
				// 解析种子文件
				metaInfo, err := metainfo.LoadFromFile(arg)
				if err != nil {
					return nil, fmt.Errorf("error loading torrent file: %q: %s", arg, err)
				}
				// 加载种子文件
				t, err := cli.AddTorrent(metaInfo)
				if err != nil {
					return nil, fmt.Errorf("adding torrent: %w", err)
				}
				return t, nil
			}
		}()
		if err != nil {
			return fmt.Errorf("adding torrent for %q: %w", arg, err)
		}
		t.SetOnWriteChunkError(func(err error) {
			err = fmt.Errorf("error writing chunk for %v: %w", t, err)
			fataErr(err)
		})
		t.AddPeers(testPeers)
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case <-t.GotInfo():
			}
			// 种子下载完成后，将种子文件保存到本地
			if flags.SaveMetainfos {
				path := fmt.Sprintf("%v.torrent", t.InfoHash().HexString())
				// 将给定元信息对象写入到文件中
				err := writeMetainfoToFile(t.Metainfo(), path)
				if err == nil {
					log.Printf("wrote %q", path)
				} else {
					log.Printf("error writing %q: %s", path, err)
				}
			}
			if len(flags.File) == 0 {
				t.DownloadAll()
				wg.Add(1)
				go func() {
					defer wg.Done()
					// 监控特定范围内torrent数据的下载进度， 在后台持续检查块的状态，直到所有目前块的下载都达到要求的完成状态
					waitForPieces(ctx, t, 0, t.NumPieces())
				}()
				done := make(chan struct{})
				go func() {
					defer close(done)
					if flags.LinearDiscard {
						r := t.NewReader()
						io.Copy(io.Discard, r)
						r.Close()
					}
				}()
				select {
				case <-done:
				case <-ctx.Done():
				}
			} else {
				// for _, f := range t.Files() {

				// }
			}
		}()
	}
	return nil
}

// 监控特定范围内torrent数据的下载进度， 在后台持续检查块的状态，直到所有目前块的下载都达到要求的完成状态
func waitForPieces(ctx context.Context, t *torrent.Torrent, beginIndex, endIndex int) {
	sub := t.SubscribePieceStateChanges()
	defer sub.Close()
	expected := storage.Completion{
		Complete: true,
		Ok:       true,
	}
	pending := make(map[int]struct{})
	for i := beginIndex; i < endIndex; i++ {
		if t.Piece(i).State().Completion != expected {
			pending[i] = struct{}{}
		}
	}
	for {
		if len(pending) == 0 {
			return
		}
		select {
		case ev := <-sub.Values:
			if ev.Completion == expected {
				delete(pending, ev.Index)
			}
		case <-ctx.Done():
			return
		}
	}
}

// 将给定元信息对象写入到文件中
func writeMetainfoToFile(mi metainfo.MetaInfo, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o6440)
	if err != nil {
		return err
	}
	defer f.Close()
	err = mi.Write(f)
	if err != nil {
		return err
	}
	return f.Close()
}

type stringAddr string

func (me stringAddr) String() string { return string(me) }

func resolveTestPeers(addrs []string) (ret []torrent.PeerInfo) {
	for _, ta := range addrs {
		ret = append(ret, torrent.PeerInfo{
			Addr: stringAddr(ta),
		})
	}
	return
}

// 根据用户的配置选项输出 torrent 客户端的统计信息和状态报告
func optputStats(cl *torrent.Client, args downloadFlags) {
	if !args.Stats {
		return
	}

	expvar.Do(func(kv expvar.KeyValue) {
		fmt.Printf("%s: %s\n", kv.Key, kv.Value)
	})
	cl.WriteStatus(os.Stdout)
}
