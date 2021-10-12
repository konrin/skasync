# 🌪 Skasync
It is a tool to quickly sync files to Kubernetes pods for a development environment.

Skasync has two modes of operation.

## WATCHER  mode
Skasync starts listening for changes in files in the working directory. All changes are accumulated during the debounce and synchronized with the endpoints (copy / delete).
```bash
skasync watcher -c path/to/config.json
```

## SYNC mode
The files of the selected working directories are copied to the specified endpoints.
```bash
# skasync sync [in|out] [all|endpoint1,endpoint2,...] path1,path2,...
# [in|out] - copy direction
#   in - copy from locale to endpoints
#   out - not implementation
# [all|endpoint1,endpoint2,...] - target to copy
#   all - copying will occur to all endpoints specified in the config
#   endpoint1,endpoint2, ... - comma-separated list of endpoints to send files
# path1,path2,... - listing the paths within the working directory to be copied to the endpoints
skasync sync in all -c path/to/config.json
```

### Example config file
```jsonc
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

## Why not use dev mode in skaffold?!
The main problem is too long counting changes on a large project.

Skaffold has three sync modes (manual / notification / polling). Regardless of the mode, each event triggers a FULL crawl of all files in the working directory to generate a list of changes. For each artifact separately!

In addition, this approach reduces the ssd resource.

Happy coding 🍻
