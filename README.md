# sweep: centralize crash dumps

Small Go program to upload files to GCS, announce them in Slack, then delete them.

Scans through a directory and uploads anything with an old enough `mtime`.

Requires the `SLACK_TOKEN` environment variable, and Google Cloud authentication. Options in `-help`.

A Docker image is published to [DockerHub](https://hub.docker.com/repository/docker/pcarrier/sweep).
