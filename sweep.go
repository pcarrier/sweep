package main

import (
	"cloud.google.com/go/storage"
	"compress/gzip"
	"context"
	"errors"
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
	bucket    = flag.String("bucket", "", "GCS bucket to transfer to")
	interval  = flag.Duration("interval", time.Minute, "Time interval to check for new uploads")
	minAge    = flag.Duration("min-age", time.Minute, "Minimum mtime age for a file to be considered for transfer")
	root      = flag.String("root", "/var/crash", "Directory to transfer")
	channelId = flag.String("slack-channel-id", "C05SP2XRK7G", "Slack channel for announcements")
)

func main() {
	flag.Parse()

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Couldn't read hostname: %v", err)
	}

	slackToken, found := os.LookupEnv("SLACK_TOKEN")
	if !found {
		log.Fatal("Please set SLACK_TOKEN")
	}
	chat := slack.New(slackToken)

	if *bucket == "" {
		log.Fatalf("Please pass -bucket")
	}
	gcs, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("Couldn't initialize GCS: %v", err)
	}

	ticker := time.NewTicker(*interval)
	for range ticker.C {
		startedAt := time.Now()
		newestUploaded := startedAt.Add(-*minAge)

		if err := filepath.WalkDir(*root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			fi, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("stat error: %w", err)
			}

			mtime := fi.ModTime().UTC()
			if mtime.After(newestUploaded) {
				log.Printf("Skipping %s, too new", path)
			} else {
				log.Printf("Uploading %s (%d bytes)", path, fi.Size())

				shortPath, _ := strings.CutPrefix(path, "/")
				gcsPath := fmt.Sprintf("%s/%s@%04d-%02d-%02dT%02d:%02d:%02dZ.gz",
					hostname, shortPath,
					mtime.Year(), mtime.Month(), mtime.Day(),
					mtime.Hour(), mtime.Minute(), mtime.Second())
				writer := gcs.
					Bucket(*bucket).
					Object(gcsPath).
					NewWriter(context.Background())
				gzWriter := gzip.NewWriter(writer)
				file, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("open error: %w", err)
				}
				if _, err = io.Copy(gzWriter, file); err != nil {
					return fmt.Errorf("copy error: %w", err)
				}
				if err = gzWriter.Close(); err != nil {
					return fmt.Errorf("gzip close error: %w", err)
				}
				if err = writer.Close(); err != nil {
					return fmt.Errorf("GCS close error: %w", err)
				}
				if err = file.Close(); err != nil {
					return fmt.Errorf("close error: %w", err)
				}
				for {
					if _, _, err = chat.PostMessage(*channelId, slack.MsgOptionText(
						fmt.Sprintf("Uploaded `gs://%s/%s` (%d bytes)",
							*bucket,
							gcsPath,
							fi.Size()), false)); err != nil {
						var rlError *slack.RateLimitedError
						if errors.As(err, &rlError) {
							log.Printf("Rate limited, sleeping %s", rlError.RetryAfter)
							time.Sleep(rlError.RetryAfter)
							break
						}
						return fmt.Errorf("error from slack: %w", err)
					} else {
						break
					}
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
