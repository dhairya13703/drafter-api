package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

type DrafterAPI struct {
	router *gin.Engine
}

type VMConfig struct {
	Name      string `json:"name"`
	Memory    string `json:"memory"`
	CPUs      int    `json:"cpus"`
	DiskSize  string `json:"disk_size"`
	ImagePath string `json:"image_path"`
}

func runCommandWithOutput(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error: %v, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func NewDrafterAPI() *DrafterAPI {
	api := &DrafterAPI{
		router: gin.Default(),
	}
	api.setupRoutes()
	return api
}

func (api *DrafterAPI) setupRoutes() {
	api.router.POST("/vm/create", api.createVM)
	api.router.POST("/vm/start/:name", api.startVM)
	api.router.POST("/vm/stop/:name", api.stopVM)
	api.router.GET("/vm/status/:name", api.getVMStatus)
	api.router.POST("/vm/migrate/:name", api.migrateVM)
}

func (api *DrafterAPI) createVM(c *gin.Context) {
	var config VMConfig
	if err := c.BindJSON(&config); err != nil {
		log.Printf("Error parsing request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Creating VM: %s", config.Name)

	// Create directories
	cmd := exec.Command("mkdir", "-p", "out/blueprint", "out/package", "out/instance-0/overlay", "out/instance-0/state")
	if out, err := runCommandWithOutput(cmd); err != nil {
		log.Printf("Error creating directories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create directories: %v", err)})
		return
	} else {
		log.Printf("Created directories: %s", out)
	}

	// Download DrafterOS
	downloadURL := fmt.Sprintf("https://github.com/loopholelabs/drafter/releases/latest/download/drafteros-oci-%s_pvm.tar.zst", runtime.GOARCH)
	log.Printf("Downloading DrafterOS from: %s", downloadURL)
	downloadCmd := exec.Command("curl", "-L", "-o", "out/drafteros-oci.tar.zst", downloadURL)
	if out, err := runCommandWithOutput(downloadCmd); err != nil {
		log.Printf("Error downloading DrafterOS: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to download DrafterOS: %v", err)})
		return
	} else {
		log.Printf("Downloaded DrafterOS: %s", out)
	}

	// Extract DrafterOS blueprint
	log.Printf("Extracting DrafterOS blueprint")
	extractCmd := exec.Command("drafter-packager", "--package-path", "out/drafteros-oci.tar.zst", "--extract", "--devices", `[{"name":"kernel","path":"out/blueprint/vmlinux"},{"name":"disk","path":"out/blueprint/rootfs.ext4"}]`)
	if out, err := runCommandWithOutput(extractCmd); err != nil {
		log.Printf("Error extracting DrafterOS: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to extract DrafterOS: %v", err)})
		return
	} else {
		log.Printf("Extracted DrafterOS: %s", out)
	}

	// Start NAT service
	log.Printf("Starting NAT service")
	natCmd := exec.Command("drafter-nat", "--host-interface", "eth0")
	if err := natCmd.Start(); err != nil {
		log.Printf("Error starting NAT service: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start NAT service: %v", err)})
		return
	}

	log.Printf("Waiting 15 seconds for NAT to initialize")
	time.Sleep(15 * time.Second)

	// Start snapshotter
	log.Printf("Starting snapshotter")
	snapshotterCmd := exec.Command("drafter-snapshotter", "--netns", "ark0", "--cpu-template", "T2A", "--memory-size", "2048", "--devices", `[{"name":"state","output":"out/package/state.bin"},{"name":"memory","output":"out/package/memory.bin"},{"name":"kernel","input":"out/blueprint/vmlinux","output":"out/package/vmlinux"},{"name":"disk","input":"out/blueprint/rootfs.ext4","output":"out/package/rootfs.ext4"},{"name":"config","output":"out/package/config.json"}]`)
	if err := snapshotterCmd.Start(); err != nil {
		log.Printf("Error starting snapshotter: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start snapshotter: %v", err)})
		return
	}

	log.Printf("VM creation initiated successfully: %s", config.Name)
	c.JSON(http.StatusOK, gin.H{"message": "VM creation initiated", "name": config.Name})
}

func (api *DrafterAPI) startVM(c *gin.Context) {
	name := c.Param("name")
	log.Printf("Starting VM: %s", name)

	// Start peer service
	log.Printf("Starting peer service")
	peerCmd := exec.Command("drafter-peer", "--netns", "ark0", "--raddr", "", "--laddr", ":1337")
	if err := peerCmd.Start(); err != nil {
		log.Printf("Error starting peer service: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start peer service"})
		return
	}

	log.Printf("Waiting 5 seconds for peer to initialize")
	time.Sleep(5 * time.Second)

	// Start forwarder
	log.Printf("Starting forwarder")
	forwarderCmd := exec.Command("drafter-forwarder", "--port-forwards", `[{"netns":"ark0","internalPort":"6379","protocol":"tcp","externalAddr":"127.0.0.1:3333"}]`)
	if err := forwarderCmd.Start(); err != nil {
		log.Printf("Error starting forwarder: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start forwarder"})
		return
	}

	log.Printf("VM started successfully: %s", name)
	c.JSON(http.StatusOK, gin.H{"message": "VM started", "name": name})
}

func (api *DrafterAPI) stopVM(c *gin.Context) {
	name := c.Param("name")
	log.Printf("Stopping VM: %s", name)

	// Stop all drafter services
	cmd := exec.Command("pkill", "-f", "drafter-")
	if out, err := runCommandWithOutput(cmd); err != nil {
		log.Printf("Error stopping services: %v", err)
	} else {
		log.Printf("Services stopped: %s", out)
	}

	log.Printf("VM stopped successfully: %s", name)
	c.JSON(http.StatusOK, gin.H{"message": "VM stopped", "name": name})
}

func (api *DrafterAPI) getVMStatus(c *gin.Context) {
	name := c.Param("name")
	log.Printf("Getting status for VM: %s", name)

	// Check if services are running
	natRunning := exec.Command("pgrep", "-f", "drafter-nat").Run() == nil
	peerRunning := exec.Command("pgrep", "-f", "drafter-peer").Run() == nil
	forwarderRunning := exec.Command("pgrep", "-f", "drafter-forwarder").Run() == nil

	status := gin.H{
		"name": name,
		"services": gin.H{
			"nat":       natRunning,
			"peer":      peerRunning,
			"forwarder": forwarderRunning,
		},
	}

	log.Printf("Status for VM %s: %v", name, status)
	c.JSON(http.StatusOK, status)
}

func (api *DrafterAPI) migrateVM(c *gin.Context) {
	name := c.Param("name")
	log.Printf("Migration requested for VM: %s (not implemented)", name)
	c.JSON(http.StatusOK, gin.H{"message": "Migration not implemented yet", "name": name})
}

func main() {
	log.Printf("Starting Drafter API server")
	api := NewDrafterAPI()
	if err := api.router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
