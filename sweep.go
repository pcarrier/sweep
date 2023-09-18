package main

import (
	"cloud.google.com/go/storage"
	"context"
	"flag"
	"fmt"
	"github.com/slack-go/slack"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	bucket    = flag.String("bucket", "", "GCS bucket for uploads")
	interval  = flag.Duration("interval", time.Minute, "Time interval to check for new uploads")
	minAge    = flag.Duration("minage", time.Minute, "Minimum mtime age for a file to be considered for transfer")
	root      = flag.String("root", "/var/crash", "Directory to transfer")
	channelId = flag.String("slackchannelid", "C17LW51GR", "Slack channel for announcements")
)

func main() {
	flag.Parse()

	slackToken, found := os.LookupEnv("SLACK_TOKEN")
	if !found {
		log.Fatal("Please set SLACK_TOKEN")
	}

	chat := slack.New(slackToken)
	gcs, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("Couldn't initialize GCS: %v", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Couldn't read hostname: %v", err)
	}

	ticker := time.NewTicker(*interval)
	for range ticker.C {
		startedAt := time.Now()
		newestUploaded := startedAt.Add(-*minAge)

		if err := filepath.WalkDir(*root, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}

			fi, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("stat error: %w", err)
			}

			mtime := fi.ModTime()
			if mtime.After(newestUploaded) {
				log.Printf("Skipping %s, too new", path)
			} else {
				log.Printf("Uploading %s", path)

				shortPath, _ := strings.CutPrefix(path, "/")
				gcsPath := fmt.Sprintf("%04d/%02d/%02d/%s/%s",
					mtime.Year(), mtime.Month(), mtime.Day(),
					hostname, shortPath)
				writer := gcs.
					Bucket(*bucket).
					Object(gcsPath).
					NewWriter(context.Background())
				file, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("open error: %w", err)
				}
				if _, err = io.Copy(writer, file); err != nil {
					return fmt.Errorf("copy error: %w", err)
				}
				if err = writer.Close(); err != nil {
					return fmt.Errorf("GCS close error: %w", err)

				}
				if err = file.Close(); err != nil {
					return fmt.Errorf("close error: %w", err)
				}
				if _, _, err = chat.PostMessage(*channelId,
					slack.MsgOptionText(
						fmt.Sprintf("Uploaded `gs://%s/%s`",
							*bucket,
							gcsPath), false)); err != nil {
					return fmt.Errorf("error from slack: %w", err)
				}
				if err = os.Remove(path); err != nil {
					return fmt.Errorf("remove error: %w", err)
				}
			}
			return nil
		}); err != nil {
			log.Fatalf("Walk failed: %v", err)
		}
	}
}
