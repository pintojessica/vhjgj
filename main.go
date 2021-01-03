package main

import (
	"flag"
	"log"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/libretro/ludo/audio"
	"github.com/libretro/ludo/core"
	"github.com/libretro/ludo/history"
	"github.com/libretro/ludo/input"
	"github.com/libretro/ludo/menu"
	"github.com/libretro/ludo/netplay"
	ntf "github.com/libretro/ludo/notifications"
	"github.com/libretro/ludo/playlists"
	"github.com/libretro/ludo/scanner"
	"github.com/libretro/ludo/settings"
	"github.com/libretro/ludo/state"
	"github.com/libretro/ludo/video"
)

const tickRATE = 1.0 / 60.0
const maxFrameSkip = 25

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

func runLoop(vid *video.Video, m *menu.Menu) {
	currTime := time.Now()
	prevTime := time.Now()
	lag := float64(0)
	for !vid.Window.ShouldClose() {
		currTime = time.Now()
		dt := float64(currTime.Sub(prevTime)) / 1000000000

		glfw.PollEvents()
		m.ProcessHotkeys()
		vid.ResizeViewport()
		m.UpdatePalette()

		state.Global.ForcePause = vid.Window.GetKey(glfw.KeySpace) == glfw.Press

		// Cap number of Frames that can be skipped so lag doesn't accumulate
		lag = math.Min(lag+dt, tickRATE*maxFrameSkip)

		for lag >= tickRATE {
			netplay.Update(input.Poll, gameUpdate)
			lag -= tickRATE
		}

		vid.Render()
		glfw.SwapInterval(0)
		vid.Window.SwapBuffers()
		prevTime = currTime
	}
}

func gameUpdate() {
	// if input.LocalPlayerPort == 0 {
	// 	log.Println("----> updating", state.Global.Tick, gameGetSyncData(), netplay.GetLocalInputState(state.Global.Tick))
	// } else {
	// 	log.Println("----> updating", state.Global.Tick, gameGetSyncData(), netplay.GetRemoteInputState(state.Global.Tick))
	// }
	state.Global.Core.Run()
	//log.Println("----> done updating")
	if state.Global.Core.FrameTimeCallback != nil {
		state.Global.Core.FrameTimeCallback.Callback(state.Global.Core.FrameTimeCallback.Reference)
	}
	if state.Global.Core.AudioCallback != nil {
		state.Global.Core.AudioCallback.Callback()
	}
}

func main() {
	err := settings.Load()
	if err != nil {
		log.Println("[Settings]: Loading failed:", err)
		log.Println("[Settings]: Using default settings")
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.StringVar(&state.Global.CorePath, "L", "", "Path to the libretro core")
	flag.BoolVar(&state.Global.Verbose, "v", false, "Verbose logs")
	flag.BoolVar(&state.Global.LudOS, "ludos", false, "Expose the features related to LudOS")
	flag.BoolVar(&netplay.Listen, "listen", false, "For the netplay server")
	flag.BoolVar(&netplay.Join, "join", false, "For the netplay client")
	flag.Parse()
	args := flag.Args()

	var gamePath string
	if len(args) > 0 {
		gamePath = args[0]
	}

	if err := glfw.Init(); err != nil {
		log.Fatalln("Failed to initialize glfw", err)
	}
	defer glfw.Terminate()

	state.Global.DB, err = scanner.LoadDB(settings.Current.DatabaseDirectory)
	if err != nil {
		log.Println("Can't load game database:", err)
	}

	playlists.Load()

	history.Load()

	vid := video.Init(settings.Current.VideoFullscreen)

	audio.Init()

	m := menu.Init(vid)

	core.Init(vid)

	input.Init(vid)

	if len(state.Global.CorePath) > 0 {
		err := core.Load(state.Global.CorePath)
		if err != nil {
			panic(err)
		}
	}

	if len(gamePath) > 0 {
		err := core.LoadGame(gamePath)
		if err != nil {
			ntf.DisplayAndLog(ntf.Error, "Menu", err.Error())
		} else {
			m.WarpToQuickMenu()
		}
	}

	// No game running? display the menu
	state.Global.MenuActive = !state.Global.CoreRunning

	runLoop(vid, m)

	// Unload and deinit in the core.
	core.Unload()
}
