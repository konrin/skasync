# 🌪 Skasync
It is a tool to quickly sync files to Kubernetes pods for a development environment.

Skasync has two modes of operation.

## WATCHER  mode
In this mode, skasync starts listening to the working directory for changes in files. All changes are accumulated during the debounce time, after which synchronization occurs to the endpoints (copy / delete).
```bash
skasync watcher -c path/to/config.json
```

## SYNC mode
This mode copies the selected working directory paths to the specified endpoints.
```bash
# skasync sync [in|out] [all|endpoint1, endpoint2,...] path1,path2,...
# [in|out] - copy direction
#   in - copy from locale to endpoints
#   out - not implementation
# [all|endpoint1,endpoint2,...] - target to copy
#   all - copying will occur to all endpoints specified in the config
#   endpoint1,endpoint2, ... - comma-separated list of endpoints to send files
# path1,path2,... - Listing the paths within the working directory to be copied to the endpoints
skasync sync in all -c path/to/config.json
```

### Example config file
```json
{
    // Path to work directory (relative to the location of the configuration file or full path)
    "RootDir": ".",
    "Artifacts": {
        "dev": {
            // Docker image name
            "Image": "app/dev",
            // Path to the working directory of the application in the container
            "RootDir": "/app",
            // Path to dockerfile (relative to the location of the working directory or full path)
            "DockerfileDir": "dev/docker"
        }
    },
    "Endpoints": {
        "nginx": {
            // Artifact key
            "Artifact": "dev",
            // Pod label
            "Selector": "app=php-nginx",
            // Container name in pod
            "Container": "php"
        },
        "workers": {
            "Artifact": "dev",
            "Selector": "app=php-workers",
            "Container": "php"
        }
    },
    "Sync": {
        // Delay time for collecting modified files for synchronization (in ms)
        "Debounce": 1000
    },
    "Git": {
        // Turns on git state tracking for more information on changed files (needed for larger checkouts)
        "EnableWatching": true
    }
}
```

## Installing

### Linux
```bash
curl -Lo skasync https://github.com/konrin/skasync/releases/latest/download/skasync-linux-amd64 && chmod +x skasync && sudo mv skasync /usr/local/bin
```

### macOS
```bash
curl -Lo skasync https://github.com/konrin/skasync/releases/latest/download/skasync-darwin-amd64 && chmod +x skasync && sudo mv skasync /usr/local/bin
```

### windows
https://github.com/konrin/skasync/releases/latest/download/skasync-windows-amd64.exe

## Why not dev mode in skaffold?!
The main problem is too long counting of changes on a large project. Skaffold has three synchronization modes (manual / notify / polling), each of the modes is just a trigger to start the process of a FULL crawl through all files in the working directory (for each artifact separately!) for the subsequent calculation of the list of changes. First, this is a long time, and secondly, the ssd resource decreases.

Happy coding 🍻
