package main

import (
	"context"
	"log"
	"net/http"
	"os/exec"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create directories and setup VM
	cmd := exec.Command("mkdir", "-p", "out/blueprint", "out/package", "out/instance-0/overlay", "out/instance-0/state")
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Start NAT service
	natCmd := exec.Command("drafter-nat", "--host-interface", "eth0")
	if err := natCmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start NAT service"})
		return
	}

	// Wait for NAT to initialize
	time.Sleep(15 * time.Second)

	// Start snapshotter
	snapshotterCmd := exec.Command("drafter-snapshotter", "--netns", "ark0", "--cpu-template", "T2")
	if err := snapshotterCmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start snapshotter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "VM creation initiated", "name": config.Name})
}

func (api *DrafterAPI) startVM(c *gin.Context) {
	name := c.Param("name")
	
	// Start peer service
	peerCmd := exec.Command("drafter-peer", "--netns", "ark0", "--raddr", "", "--laddr", ":1337")
	if err := peerCmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start peer service"})
		return
	}

	// Wait for peer to initialize
	time.Sleep(5 * time.Second)

	// Start forwarder
	forwarderCmd := exec.Command("drafter-forwarder", "--port-forwards", `[{"netns":"ark0","internalPort":"6379","protocol":"tcp","externalAddr":"127.0.0.1:3333"}]`)
	if err := forwarderCmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start forwarder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "VM started", "name": name})
}

func (api *DrafterAPI) stopVM(c *gin.Context) {
	name := c.Param("name")
	
	// Stop all drafter services
	exec.Command("pkill", "-f", "drafter-").Run()

	c.JSON(http.StatusOK, gin.H{"message": "VM stopped", "name": name})
}

func (api *DrafterAPI) getVMStatus(c *gin.Context) {
	name := c.Param("name")
	
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

	c.JSON(http.StatusOK, status)
}

func (api *DrafterAPI) migrateVM(c *gin.Context) {
	name := c.Param("name")
	// TODO: Implement VM migration logic using drafter's migration capabilities
	c.JSON(http.StatusOK, gin.H{"message": "Migration not implemented yet", "name": name})
}

func main() {
	api := NewDrafterAPI()
	if err := api.router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
