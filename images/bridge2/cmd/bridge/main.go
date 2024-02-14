package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ajbouh/substrate/images/bridge2/tracks"
	"github.com/ajbouh/substrate/images/bridge2/transcribe"
	"github.com/ajbouh/substrate/images/bridge2/ui"
	"github.com/ajbouh/substrate/images/bridge2/vad"
	"github.com/ajbouh/substrate/images/bridge2/webrtc/js"
	"github.com/ajbouh/substrate/images/bridge2/webrtc/local"
	"github.com/ajbouh/substrate/images/bridge2/webrtc/sfu"
	"github.com/ajbouh/substrate/images/bridge2/webrtc/trackstreamer"
	"github.com/fxamacker/cbor/v2"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
	"tractor.dev/toolkit-go/engine"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/daemon"
)

func main() {
	format := beep.Format{
		SampleRate:  beep.SampleRate(16000),
		NumChannels: 1,
		Precision:   4,
	}
	fatal(speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)))

	engine.Run(
		Main{
			format: format,
		},
		vad.New(vad.Config{
			SampleRate:   format.SampleRate.N(time.Second),
			SampleWindow: 24 * time.Second,
		}),
		transcribe.Agent{
			Endpoint: getEnv("BRIDGE_TRANSCRIBE_URL", "http://localhost:8090/v1/transcribe"),
		},
		eventLogger{
			exclude: []string{"audio"},
		},
	)
}

var cborenc = func() cbor.EncMode {
	opts := cbor.CoreDetEncOptions()
	opts.Time = cbor.TimeRFC3339
	em, err := opts.EncMode()
	fatal(err)
	return em
}()

type eventLogger struct {
	exclude []string
}

func (l eventLogger) HandleEvent(e tracks.Event) {
	for _, t := range l.exclude {
		if e.Type == t {
			return
		}
	}
	log.Printf("event: %s %s %s", e.Type, e.ID, time.Duration(e.Start))
}

type Main struct {
	EventHandlers []tracks.Handler

	sessions map[string]*Session
	format   beep.Format
	basePath string
	port     int

	Daemon *daemon.Framework

	mu sync.Mutex
}

type Session struct {
	*tracks.Session
	sfu  *sfu.Session
	peer *local.Peer
}

type View struct {
	Sessions []*tracks.SessionInfo
	Session  *Session
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func must[T any](t T, err error) T {
	fatal(err)
	return t
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func parsePort(port string) int {
	port16 := must(strconv.ParseUint(port, 10, 16))
	return int(port16)
}

func (m *Main) Initialize() {
	basePath := os.Getenv("SUBSTRATE_URL_PREFIX")
	// ensure the path starts and ends with a slash for setting <base href>
	m.basePath = must(url.JoinPath("/", basePath, "/"))
	m.port = parsePort(getEnv("PORT", "8080"))
}

func (m *Main) InitializeCLI(root *cli.Command) {
	// a workaround for an unresolved issue in toolkit-go/engine
	// for figuring out if its a CLI or a daemon program...
	root.Run = func(ctx *cli.Context, args []string) {
		if err := m.Daemon.Run(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func (m *Main) TerminateDaemon(ctx context.Context) error {
	for _, sess := range m.sessions {
		if err := saveSession(sess); err != nil {
			return err
		}
	}
	return nil
}

func saveSession(sess *Session) error {
	b, err := cborenc.Marshal(sess)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("./sessions/%s/session", sess.ID)
	if err := os.WriteFile(filename, b, 0644); err != nil {
		return err
	}
	// for debugging!
	// b, err = json.Marshal(sess)
	// if err != nil {
	// 	return err
	// }
	// filename = fmt.Sprintf("./sessions/%s/session.json", id)
	// if err := os.WriteFile(filename, b, 0644); err != nil {
	// 	return err
	// }
	return nil
}

func (m *Main) SavedSessions() (info []*tracks.SessionInfo, err error) {
	root := "./sessions"
	dir, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, fi := range dir {
		if fi.IsDir() {
			log.Printf("reading session %s", fi.Name())
			sess, err := tracks.LoadSessionInfo(root, fi.Name())
			if err != nil {
				log.Printf("error reading session %s: %s", fi.Name(), err)
				continue
			}
			info = append(info, sess)
		}
	}
	sort.Slice(info, func(i, j int) bool {
		return info[i].Start.After(info[j].Start)
	})
	return
}

func (m *Main) StartSession(sess *Session) {
	var err error
	sess.peer, err = local.NewPeer(fmt.Sprintf("ws://localhost:%d/sessions/%s?sfu", m.port, sess.ID)) // FIX: hardcoded host
	fatal(err)
	sess.peer.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		sessTrack := sess.NewTrack(m.format)

		log.Printf("got track %s %s", track.ID(), track.Kind())
		if track.Kind() != webrtc.RTPCodecTypeAudio {
			return
		}
		ogg, err := oggwriter.New(fmt.Sprintf("./sessions/%s/track-%s.ogg", sess.ID, track.ID()), uint32(m.format.SampleRate.N(time.Second)), uint16(m.format.NumChannels))
		fatal(err)
		defer ogg.Close()
		rtp := trackstreamer.Tee(track, ogg)
		s, err := trackstreamer.New(rtp, m.format)
		fatal(err)

		chunkSize := sessTrack.AudioFormat().SampleRate.N(100 * time.Millisecond)
		for {
			// since Track.AddAudio expects finite segments, split it into chunks of
			// a smaller size we can append incrementally
			chunk := beep.Take(chunkSize, s)
			sessTrack.AddAudio(chunk)
			fatal(chunk.Err())
		}
	})
	sess.peer.HandleSignals()
}

// Return a channel which will be notified when the session receives a new
// event. Designed to debounce handling for one update at a time. The channel
// will be closed when the context is cancelled to allow "range" loops over
// the updates.
func sessionUpdateHandler(ctx context.Context, sess *Session) chan struct{} {
	ch := make(chan struct{}, 1)
	h := tracks.HandlerFunc(func(e tracks.Event) {
		if e.Type == "audio" {
			// if this is a transient event like "audio" we don't need to save
			return
		}
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	go func() {
		<-ctx.Done()
		sess.Unlisten(h)
		close(ch)
	}()
	sess.Listen(h)
	return ch
}

func (m *Main) loadSession(ctx context.Context, id string) (*Session, error) {
	m.mu.Lock()
	sess := m.sessions[id]
	m.mu.Unlock()
	if sess != nil {
		log.Println("found session in cache", id)
		return sess, nil
	}
	log.Println("loading session from disk", id)
	trackSess, err := tracks.LoadSession("./sessions", id)
	if err != nil {
		// TODO handle not found error
		return nil, err
	}
	sess = m.addSession(ctx, trackSess)
	return sess, nil
}

func (m *Main) addSession(ctx context.Context, trackSess *tracks.Session) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess := m.sessions[string(trackSess.ID)]; sess != nil {
		return sess
	}
	sess := &Session{
		sfu:     sfu.NewSession(),
		Session: trackSess,
	}
	// For older sessions we may want to leave them read-only, at least by
	// default. We could give them the option to start recording again, but they
	// may not want new audio to be automatically recorded when they look it up.
	for _, h := range m.EventHandlers {
		sess.Listen(h)
	}
	go func() {
		for range sessionUpdateHandler(ctx, sess) {
			log.Printf("saving session")
			fatal(saveSession(sess))
		}
	}()
	m.sessions[string(sess.ID)] = sess
	fatal(os.MkdirAll(fmt.Sprintf("./sessions/%s", sess.ID), 0744))
	go m.StartSession(sess)
	return sess
}

func (m *Main) Serve(ctx context.Context) {
	m.sessions = make(map[string]*Session)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		sess := m.addSession(ctx, tracks.NewSession())
		http.Redirect(w, r, path.Join(m.basePath, "sessions", string(sess.ID)), http.StatusFound)
	})

	http.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		sessID := filepath.Base(r.URL.Path)
		sess, err := m.loadSession(ctx, sessID)
		if err != nil {
			// TODO different error if we failed to load vs not found
			log.Printf("error loading session %s: %s", sessID, err)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		updateCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		updateCh := sessionUpdateHandler(updateCtx, sess)
		select {
		case updateCh <- struct{}{}: // trigger initial update
		default:
		}

		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Print("upgrade:", err)
				return
			}
			if r.URL.RawQuery == "sfu" {
				peer, err := sess.sfu.AddPeer(conn)
				if err != nil {
					log.Print("peer:", err)
					return
				}
				peer.HandleSignals()
			}
			if r.URL.RawQuery == "data" {
				for range updateCh {
					// TODO check periodically for new sessions even if there's not an
					// update on this session
					names, err := m.SavedSessions()
					fatal(err)
					data, err := cborenc.Marshal(View{
						Sessions: names,
						Session:  sess,
					})
					fatal(err)
					if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
						log.Println("data:", err)
						return
					}
				}
			}
			return
		}

		content, err := fs.ReadFile(ui.Dir, "session.html")
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		content = bytes.Replace(content,
			[]byte("<head>"),
			[]byte(`<head><base href="`+m.basePath+`">`),
			1)
		b := bytes.NewReader(content)
		http.ServeContent(w, r, "session.html", time.Now(), b)
	})

	http.Handle("/webrtc/", http.StripPrefix("/webrtc", http.FileServer(http.FS(js.Dir))))
	http.Handle("/ui/", http.StripPrefix("/ui", http.FileServer(http.FS(ui.Dir))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, path.Join(m.basePath, "sessions"), http.StatusFound)
	})

	log.Printf("running on http://localhost:%d ...", m.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", m.port), nil))
}
