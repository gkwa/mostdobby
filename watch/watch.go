package watch

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

type DirectoryConfig struct {
	WorkFunc     WorkFunc
	MaxEvents    int
	EventTimeout time.Duration
}

type WorkFunc func(dir string)

func ProcessDirectoryChanges(dir string, config DirectoryConfig) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", dir)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	err = watcher.Add(dir)
	if err != nil {
		return fmt.Errorf("failed to add directory to watcher: %v", err)
	}

	slog.Info("watching directory", "path", dir)

	done := make(chan bool)

	eventCount := 0
	lastEventTime := time.Now()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				switch {
				case event.Op&fsnotify.Write == fsnotify.Write:
				case event.Op&fsnotify.Create == fsnotify.Create:
				case event.Op&fsnotify.Remove == fsnotify.Remove:
				case event.Op&fsnotify.Chmod == fsnotify.Chmod:
				default:
					continue
				}

				currentTime := time.Now()
				remainingTime := (config.EventTimeout - currentTime.Sub(lastEventTime)).Truncate(time.Second)
				if currentTime.Sub(lastEventTime) > config.EventTimeout {
					eventCount = 1
				} else {
					eventCount++
					if eventCount > config.MaxEvents {
						slog.Debug("too many events, suppressing...",
							"count", eventCount,
							"max", config.MaxEvents,
							"timeout", config.EventTimeout,
							"lastEventTime", lastEventTime,
							"currentTime", currentTime,
							"suppression time remaining", remainingTime.String(),
						)
						slog.Info("suppression stats",
							"op", event.Op,
							"timeout", config.EventTimeout,
							"suppression time remaining", remainingTime.String(),
						)
						continue
					}
				}
				lastEventTime = currentTime

				slog.Debug("File event", "op", event.Op, "fname", event.Name, "dir", dir)
				config.WorkFunc(dir)

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("watcher error", "err", err)
			}
		}
	}()

	<-done

	return nil
}

func RunTest(dir string) error {
	config := DirectoryConfig{
		WorkFunc: func(dir string) {
			slog.Info("custom work function called with directory:", "path", dir)
		},
		MaxEvents:    1,
		EventTimeout: 5 * time.Second,
	}
	return ProcessDirectoryChanges(dir, config)
}
