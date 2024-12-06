# Drafter REST API

This is a REST API wrapper around the Drafter VM management system. It provides simple HTTP endpoints to manage VMs using Drafter.

## Prerequisites

- Go 1.21 or later
- Drafter binaries installed
- Root/sudo access (for running Drafter commands)

## Installation

```bash
go mod tidy
go build -o drafter-api
```

## Running the API

```bash
sudo ./drafter-api
```

## API Endpoints

### Create VM
```bash
POST /vm/create
{
    "name": "test-vm",
    "memory": "2G",
    "cpus": 2,
    "disk_size": "10G",
    "image_path": "/path/to/image"
}
```

### Start VM
```bash
POST /vm/start/:name
```

### Stop VM
```bash
POST /vm/stop/:name
```

### Get VM Status
```bash
GET /vm/status/:name
```

### Migrate VM (Not implemented yet)
```bash
POST /vm/migrate/:name
```

## Example Usage

Create a VM:
```bash
curl -X POST http://localhost:8080/vm/create \
  -H "Content-Type: application/json" \
  -d '{"name":"test-vm","memory":"2G","cpus":2,"disk_size":"10G","image_path":"/path/to/image"}'
```

Start a VM:
```bash
curl -X POST http://localhost:8080/vm/start/test-vm
```

Get VM status:
```bash
curl http://localhost:8080/vm/status/test-vm
```

Stop a VM:
```bash
curl -X POST http://localhost:8080/vm/stop/test-vm
```
