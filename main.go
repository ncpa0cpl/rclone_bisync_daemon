package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/radovskyb/watcher"
)

var wg sync.WaitGroup

func printRunHelp() {
	fmt.Println("Usage: rclone_bisync_daemon run [options]")
	fmt.Println("")
	fmt.Println("  Starts the rclone bisync daemon, this daeomn will sync local with remote in specified")
	fmt.Println("  interval and when file changes are detected.")
	fmt.Println("")
	fmt.Println("  Options:")
	fmt.Println("    --dir <path>               Path to the local directory to sync")
	fmt.Println("    --remote-dir <path>        Path to the remote directory to sync")
	fmt.Println("    --sync-interval <seconds>  How often to auto sync the directories")
	fmt.Println("    --debounce <seconds>       Sync debounce time between file changes")
}

func printRegisterHelp() {
	fmt.Println("Usage: rclone-bisync register [options]")
	fmt.Println("")
	fmt.Println("  Registers the rclone bisync daemon to systemd.")
	fmt.Println("")
	fmt.Println("  Options:")
	fmt.Println("    --dir <path>               Path to the local directory to sync")
	fmt.Println("    --remote-dir <path>        Path to the remote directory to sync")
	fmt.Println("    --sync-interval <seconds>  How often to auto sync the directories")
	fmt.Println("    --debounce <seconds>       Sync debounce time between file changes")
}

func printResyncHelp() {
	fmt.Println("Usage: rclone_bisync_daemon resync [options]")
	fmt.Println("")
	fmt.Println("  Runs a one time re-sync of the local and remote directories.")
	fmt.Println("")
	fmt.Println("  Options:")
	fmt.Println("    --dir <path>               Path to the local directory to sync")
	fmt.Println("    --remote-dir <path>        Path to the remote directory to sync")
}

func main() {
	programArgs := ParseArgs(os.Args[1:], []string{
		"--help",
	})

	switch programArgs.Input {
	case "run":
		if programArgs.HasParam("help") {
			printRunHelp()
			return
		}

		dirPath := programArgs.GetParam("dir", "")
		remotePath := programArgs.GetParam("remote-dir", "")
		interval := programArgs.GetParamInt64("sync-interval", 60*5)
		watchDebounce := programArgs.GetParamInt64("debounce", 30)
		validateDirPath(dirPath)
		validateSet(remotePath)
		runDaemon(
			dirPath,
			remotePath,
			time.Duration(interval*int64(time.Second)),
			time.Duration(watchDebounce*int64(time.Second)),
		)
	case "register":
		if programArgs.HasParam("help") {
			printRegisterHelp()
			return
		}

		dirPath := programArgs.GetParam("dir", "")
		remotePath := programArgs.GetParam("remote-dir", "")
		interval := programArgs.GetParamInt64("sync-interval", 60*5)
		watchDebounce := programArgs.GetParamInt64("debounce", 30)
		validateDirPath(dirPath)
		validateSet(remotePath)

		fmt.Println("adding deamon to systemd")
		registerToSystemd(
			dirPath,
			remotePath,
			time.Duration(interval*int64(time.Second)),
			time.Duration(watchDebounce*int64(time.Second)),
		)
	case "resync":
		if programArgs.HasParam("help") {
			printResyncHelp()
			return
		}

		dirPath := programArgs.GetParam("dir", "")
		remotePath := programArgs.GetParam("remote-dir", "")
		validateDirPath(dirPath)
		validateSet(remotePath)
		bisync(dirPath, remotePath, true)
	}

	wg.Wait()
}

func validateSet(path string) {
	if path == "" {
		fmt.Println("Directory path is required")
		panic("")
	}
}

func validateDirPath(dirPath string) {
	validateSet(dirPath)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		fmt.Printf("Directory %s does not exist\n", dirPath)
		panic("")
	}
}

func escapePath(path string) string {
	return strings.ReplaceAll(path, " ", "\\ ")
}

func registerToSystemd(
	dirPath string,
	remotePath string,
	interval time.Duration,
	watchDebounce time.Duration,
) {
	execPath, err := os.Executable()
	if err != nil {
		fmt.Println("Unable to find the path of the rclone_bisync_daemon executable")
		fmt.Println(err)
		panic("")
	}

	lines := []string{
		"[Unit]",
		"Description=rclone bisync daemon",
		"After=network.target",
		"",
		"[Service]",
		"Type=simple",
		"ExecStart=" + execPath + " run " + "--dir " + escapePath(dirPath) + " --remote-dir " + escapePath(remotePath) + " --sync-interval " + fmt.Sprintf("%d", interval/time.Second) + " --debounce " + fmt.Sprintf("%d", watchDebounce/time.Second),
		"Restart=on-failure",
		"RestartSec=5",
		"",
		"[Install]",
		"WantedBy=default.target",
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Unable to resolve the home directory")
		fmt.Println(err)
		panic("")
	}

	servicesDir := path.Join(homedir, ".config/systemd/user")

	err = os.MkdirAll(servicesDir, 0755)
	if err != nil {
		fmt.Println("Unable to create the systemd services directory")
		fmt.Println(err)
		panic("")
	}

	err = os.WriteFile(
		path.Join(servicesDir, "rclone-bisync-daemon.service"),
		[]byte(strings.Join(lines, "\n")), 0644,
	)
	if err != nil {
		fmt.Println("Unable to write the systemd service file")
		fmt.Println(err)
		panic("")
	}

	enableCmd := exec.Command("systemctl", "--user", "enable", "rclone-bisync-daemon")
	if err := enableCmd.Run(); err != nil {
		fmt.Println("Unable to enable the rclone bisync daemon")
		fmt.Println(err)
	} else {
		fmt.Println("rclone bisync daemon was successfully registered")
	}

	startCmd := exec.Command("systemctl", "--user", "start", "rclone-bisync-daemon")
	if err := startCmd.Run(); err != nil {
		fmt.Println("Unable to start the rclone bisync daemon")
		fmt.Println(err)
	}

	bisync(dirPath, remotePath, true)
}

var isSyncing = false

func when[T any](condition bool, then T, els T) T {
	if condition {
		return then
	}
	return els
}

func bisync(dirPath string, remotePath string, resync bool) {
	if isSyncing {
		return
	}

	cmdArgs := []string{
		"bisync",
		dirPath,
		remotePath,
		"--create-empty-src-dirs",
		"--compare", "size,modtime,checksum",
		"--slow-hash-sync-only",
		"--resilient",
		"-MvP",
		"--drive-skip-gdocs",
		"--fix-case",
	}

	if resync {
		cmdArgs = append(cmdArgs, "--resync")
	}

	fmt.Println("Syncing started")
	cmd := exec.Command("rclone", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println("Syncing failed")
		fmt.Println(err, "\n", string(out))
	} else {
		fmt.Println("Syncing finished")
	}

	isSyncing = false
}

func runDaemon(
	dirPath string,
	remotePath string,
	interval time.Duration,
	watchDebounce time.Duration,
) {
	fmt.Println("Starting rclone bisync daemon")

	setInterval(func() {
		bisync(dirPath, remotePath, false)
	}, interval)

	bisync(dirPath, remotePath, false)

	go setupWatcher(dirPath, remotePath, watchDebounce)
}

func setupWatcher(dirPath string, remotePath string, debounce time.Duration) {
	wg.Add(1)
	w := watcher.New()

	// SetMaxEvents to 1 to allow at most 1 event's to be received
	// on the Event channel per watching cycle.
	//
	// If SetMaxEvents is not set, the default is to send all events.
	w.SetMaxEvents(1)

	w.FilterOps(
		watcher.Rename,
		watcher.Move,
		watcher.Remove,
		watcher.Write,
		watcher.Create,
		watcher.Chmod,
	)

	var currentTimeout *Timeout
	onNextChange := func() {
		if currentTimeout != nil {
			currentTimeout.Clear()
		}

		currentTimeout = setTimeout(func() {
			bisync(dirPath, remotePath, false)
		}, debounce)
	}

	go func() {
		for {
			select {
			case <-w.Event:
				if !isSyncing {
					fmt.Println("received fs event")
					onNextChange()
				}
			case err := <-w.Error:
				fmt.Println(err)
			case <-w.Closed:
				wg.Done()
				return
			}
		}
	}()

	// Watch test_folder recursively for changes.
	if err := w.AddRecursive(dirPath); err != nil {
		fmt.Println("Failed to add directory to watcher")
		fmt.Println(err)
	}

	// Start the watching process - it'll check for changes every 1000ms.
	if err := w.Start(time.Millisecond * 1000); err != nil {
		fmt.Println("File Watcher returned an error")
		fmt.Println(err)
	}
}

func setInterval(fn func(), interval time.Duration) {
	ticker := time.NewTicker(interval)
	quit := make(chan struct{})
	wg.Add(1)
	go func() {
		for {
			select {
			case <-ticker.C:
				fn()
			case <-quit:
				ticker.Stop()
				wg.Done()
				return
			}
		}
	}()
}

type Timeout struct {
	timer     *time.Timer
	isCleared chan bool
}

func (t *Timeout) Clear() {
	t.timer.Stop()
	t.isCleared <- true
}

func setTimeout(fn func(), duration time.Duration) *Timeout {
	timer := time.NewTimer(duration)
	timeout := Timeout{
		isCleared: make(chan bool),
		timer:     timer,
	}
	go (func() {
		select {
		case <-timer.C:
			fn()
		case <-timeout.isCleared:
			return
		}
	})()
	return &timeout
}
