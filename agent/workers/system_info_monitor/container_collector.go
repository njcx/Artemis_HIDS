package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"artemis_agent/shared"

	// Docker client
	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"

	// Containerd client
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"

	// CRI API (for CRI-O and containerd CRI)
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// 容器命令超时配置
const (
	ContainerAPITimeout  = 300 * time.Second // 容器 API 超时
	RuntimeDetectTimeout = 120 * time.Second // 运行时检测超时
)

// 常见的 socket 路径
var (
	dockerSockets = []string{
		"/var/run/docker.sock",
		"/run/docker.sock",
	}
	containerdSockets = []string{
		"/run/containerd/containerd.sock",
		"/var/run/containerd/containerd.sock",
	}
	crioSockets = []string{
		"/var/run/crio/crio.sock",
		"/run/crio/crio.sock",
	}
	podmanSockets = []string{
		"/run/podman/podman.sock",
		"/var/run/podman/podman.sock",
		fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid()),
	}
)

// ContainerInfo 容器信息
type ContainerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Created string `json:"created"`
	Ports   string `json:"ports"`
}

// ImageInfo 镜像信息
type ImageInfo struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	Created    string `json:"created"`
}

// ContainerCollector 容器监控采集器
type ContainerCollector struct {
	runtime      string // docker, podman, containerd, crio
	socketPath   string // 运行时 socket 路径
	dockerClient *dockerclient.Client
	ctrdClient   *containerd.Client
	criClient    runtimeapi.RuntimeServiceClient
	criImageCli  runtimeapi.ImageServiceClient
}

func NewContainerCollector() *ContainerCollector {
	return &ContainerCollector{}
}

// CollectLoop 采集循环
func (cc *ContainerCollector) CollectLoop(ctx context.Context, sendEvent func(*shared.MonitorData), mu *sync.Mutex) {
	// 检测容器运行时并初始化客户端
	cc.detectAndInitRuntime()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Printf("[container_collector] collectLoop started, runtime: %s, socket: %s, will collect after 5 seconds", cc.runtime, cc.socketPath)

	// 启动后延迟5秒采集第一次
	time.Sleep(5 * time.Second)
	log.Printf("[container_collector] Starting first collection...")

	if mu != nil {
		mu.Lock()
	}
	cc.collectContainers(sendEvent)
	if mu != nil {
		mu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[container_collector] collectLoop context cancelled, exiting")
			cc.cleanup()
			return
		case <-ticker.C:
			if mu != nil {
				mu.Lock()
			}
			cc.collectContainers(sendEvent)
			if mu != nil {
				mu.Unlock()
			}
		}
	}
}

// cleanup 清理资源
func (cc *ContainerCollector) cleanup() {
	if cc.dockerClient != nil {
		cc.dockerClient.Close()
	}
	if cc.ctrdClient != nil {
		cc.ctrdClient.Close()
	}
}

// detectAndInitRuntime 检测可用的容器运行时并初始化客户端
func (cc *ContainerCollector) detectAndInitRuntime() {
	// 1. 尝试 Docker
	if cc.tryInitDocker() {
		return
	}

	// 2. 尝试 Podman (使用 Docker 兼容 API)
	if cc.tryInitPodman() {
		return
	}

	// 3. 尝试 containerd
	if cc.tryInitContainerd() {
		return
	}

	// 4. 尝试 CRI-O
	if cc.tryInitCRIO() {
		return
	}

	cc.runtime = "none"
	log.Printf("[container_collector] No container runtime detected")
}

// tryInitDocker 尝试初始化 Docker 客户端
func (cc *ContainerCollector) tryInitDocker() bool {
	for _, sock := range dockerSockets {
		if _, err := os.Stat(sock); err != nil {
			continue
		}

		cli, err := dockerclient.NewClientWithOpts(
			dockerclient.WithHost("unix://"+sock),
			dockerclient.WithAPIVersionNegotiation(),
		)
		if err != nil {
			log.Printf("[container_collector] Failed to create Docker client for %s: %v", sock, err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), RuntimeDetectTimeout)
		_, err = cli.Ping(ctx)
		cancel()

		if err != nil {
			cli.Close()
			log.Printf("[container_collector] Docker ping failed for %s: %v", sock, err)
			continue
		}

		cc.runtime = "docker"
		cc.socketPath = sock
		cc.dockerClient = cli
		log.Printf("[container_collector] Docker detected via socket: %s", sock)
		return true
	}
	return false
}

// tryInitPodman 尝试初始化 Podman 客户端
func (cc *ContainerCollector) tryInitPodman() bool {
	for _, sock := range podmanSockets {
		if _, err := os.Stat(sock); err != nil {
			continue
		}

		cli, err := dockerclient.NewClientWithOpts(
			dockerclient.WithHost("unix://"+sock),
			dockerclient.WithAPIVersionNegotiation(),
		)
		if err != nil {
			log.Printf("[container_collector] Failed to create Podman client for %s: %v", sock, err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), RuntimeDetectTimeout)
		_, err = cli.Ping(ctx)
		cancel()

		if err != nil {
			cli.Close()
			log.Printf("[container_collector] Podman ping failed for %s: %v", sock, err)
			continue
		}

		cc.runtime = "podman"
		cc.socketPath = sock
		cc.dockerClient = cli
		log.Printf("[container_collector] Podman detected via socket: %s", sock)
		return true
	}
	return false
}

// tryInitContainerd 尝试初始化 containerd 客户端
func (cc *ContainerCollector) tryInitContainerd() bool {
	for _, sock := range containerdSockets {
		if _, err := os.Stat(sock); err != nil {
			continue
		}

		cli, err := containerd.New(sock)
		if err != nil {
			log.Printf("[container_collector] Failed to create containerd client for %s: %v", sock, err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), RuntimeDetectTimeout)
		_, err = cli.Version(ctx)
		cancel()

		if err != nil {
			cli.Close()
			log.Printf("[container_collector] containerd version check failed for %s: %v", sock, err)
			continue
		}

		cc.runtime = "containerd"
		cc.socketPath = sock
		cc.ctrdClient = cli
		log.Printf("[container_collector] containerd detected via socket: %s", sock)
		return true
	}
	return false
}

// tryInitCRIO 尝试初始化 CRI-O 客户端
func (cc *ContainerCollector) tryInitCRIO() bool {
	for _, sock := range crioSockets {
		if _, err := os.Stat(sock); err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), RuntimeDetectTimeout)
		conn, err := grpc.DialContext(ctx, "unix://"+sock,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
				return net.DialTimeout("unix", strings.TrimPrefix(addr, "unix://"), RuntimeDetectTimeout)
			}),
		)
		cancel()

		if err != nil {
			log.Printf("[container_collector] Failed to connect to CRI-O socket %s: %v", sock, err)
			continue
		}

		runtimeClient := runtimeapi.NewRuntimeServiceClient(conn)
		imageClient := runtimeapi.NewImageServiceClient(conn)

		ctx, cancel = context.WithTimeout(context.Background(), RuntimeDetectTimeout)
		_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
		cancel()

		if err != nil {
			conn.Close()
			log.Printf("[container_collector] CRI-O version check failed for %s: %v", sock, err)
			continue
		}

		cc.runtime = "crio"
		cc.socketPath = sock
		cc.criClient = runtimeClient
		cc.criImageCli = imageClient
		log.Printf("[container_collector] CRI-O detected via socket: %s", sock)
		return true
	}
	return false
}

// collectContainers 采集容器和镜像信息
func (cc *ContainerCollector) collectContainers(sendEvent func(*shared.MonitorData)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[container_collector] panic: %v", r)
		}
	}()

	log.Printf("[container_collector] collectContainers called, runtime=%s", cc.runtime)

	if cc.runtime == "none" {
		log.Printf("[container_collector] No container runtime detected, skipping")
		return
	}

	// 采集容器信息
	containers, err := cc.getContainers()
	if err != nil {
		log.Printf("[container_collector] 获取容器列表失败: %v", err)
		containers = []ContainerInfo{}
	}

	// 采集镜像信息
	images, err := cc.getImages()
	if err != nil {
		log.Printf("[container_collector] 获取镜像列表失败: %v", err)
		images = []ImageInfo{}
	}

	// 转换为 []interface{} 以便 structpb 处理
	containersList := make([]interface{}, 0, len(containers))
	for _, c := range containers {
		containersList = append(containersList, map[string]interface{}{
			"id":      c.ID,
			"name":    c.Name,
			"image":   c.Image,
			"status":  c.Status,
			"created": c.Created,
			"ports":   c.Ports,
		})
	}

	imagesList := make([]interface{}, 0, len(images))
	for _, img := range images {
		imagesList = append(imagesList, map[string]interface{}{
			"id":         img.ID,
			"repository": img.Repository,
			"tag":        img.Tag,
			"size":       img.Size,
			"created":    img.Created,
		})
	}

	dataMap := map[string]interface{}{
		"timestamp":       time.Now().Unix(),
		"runtime":         cc.runtime,
		"container_count": len(containers),
		"image_count":     len(images),
		"containers":      containersList,
		"images":          imagesList,
	}

	event := &shared.MonitorData{
		WorkerName: "container_monitor", // 使用独立的 WorkerName
		EventType:  "container_inventory",
		Timestamp:  time.Now(),
		Data:       dataMap,
	}

	sendEvent(event)
}

// getContainers 获取活跃容器列表
func (cc *ContainerCollector) getContainers() ([]ContainerInfo, error) {
	switch cc.runtime {
	case "docker", "podman":
		return cc.getDockerContainers()
	case "containerd":
		return cc.getContainerdContainers()
	case "crio":
		return cc.getCRIOContainers()
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", cc.runtime)
	}
}

// getImages 获取镜像列表
func (cc *ContainerCollector) getImages() ([]ImageInfo, error) {
	switch cc.runtime {
	case "docker", "podman":
		return cc.getDockerImages()
	case "containerd":
		return cc.getContainerdImages()
	case "crio":
		return cc.getCRIOImages()
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", cc.runtime)
	}
}

// getDockerContainers 使用 Docker SDK 获取容器
func (cc *ContainerCollector) getDockerContainers() ([]ContainerInfo, error) {
	if cc.dockerClient == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ContainerAPITimeout)
	defer cancel()

	containers, err := cc.dockerClient.ContainerList(ctx, dockertypes.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	var result []ContainerInfo
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		ports := cc.formatPorts(c.Ports)
		created := time.Unix(c.Created, 0).Format("2006-01-02 15:04:05")

		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}

		result = append(result, ContainerInfo{
			ID:      id,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			Created: created,
			Ports:   ports,
		})
	}

	return result, nil
}

// formatPorts 格式化端口映射
func (cc *ContainerCollector) formatPorts(ports []dockertypes.Port) string {
	var portStrs []string
	for _, p := range ports {
		if p.PublicPort > 0 {
			portStrs = append(portStrs, fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
		} else {
			portStrs = append(portStrs, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
		}
	}
	return strings.Join(portStrs, ", ")
}

// getDockerImages 使用 Docker SDK 获取镜像
func (cc *ContainerCollector) getDockerImages() ([]ImageInfo, error) {
	if cc.dockerClient == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ContainerAPITimeout)
	defer cancel()

	images, err := cc.dockerClient.ImageList(ctx, dockertypes.ImageListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %v", err)
	}

	var result []ImageInfo
	for _, img := range images {
		id := strings.TrimPrefix(img.ID, "sha256:")
		if len(id) > 12 {
			id = id[:12]
		}

		size := cc.formatSize(img.Size)
		created := time.Unix(img.Created, 0).Format("2006-01-02 15:04:05")

		if len(img.RepoTags) == 0 {
			result = append(result, ImageInfo{
				ID:         id,
				Repository: "<none>",
				Tag:        "<none>",
				Size:       size,
				Created:    created,
			})
		} else {
			for _, tag := range img.RepoTags {
				repo, tagName := cc.parseRepoTag(tag)
				result = append(result, ImageInfo{
					ID:         id,
					Repository: repo,
					Tag:        tagName,
					Size:       size,
					Created:    created,
				})
			}
		}
	}

	return result, nil
}

// getContainerdContainers 使用 containerd SDK 获取容器
func (cc *ContainerCollector) getContainerdContainers() ([]ContainerInfo, error) {
	if cc.ctrdClient == nil {
		return nil, fmt.Errorf("containerd client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ContainerAPITimeout)
	defer cancel()

	ctx = namespaces.WithNamespace(ctx, "default")

	containers, err := cc.ctrdClient.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	var result []ContainerInfo
	for _, c := range containers {
		info, err := c.Info(ctx)
		if err != nil {
			continue
		}

		id := c.ID()
		if len(id) > 12 {
			id = id[:12]
		}

		status := "unknown"
		task, err := c.Task(ctx, nil)
		if err == nil {
			taskStatus, err := task.Status(ctx)
			if err == nil {
				status = string(taskStatus.Status)
			}
		}

		created := info.CreatedAt.Format("2006-01-02 15:04:05")

		result = append(result, ContainerInfo{
			ID:      id,
			Name:    c.ID(),
			Image:   info.Image,
			Status:  status,
			Created: created,
		})
	}

	return result, nil
}

// getContainerdImages 使用 containerd SDK 获取镜像
func (cc *ContainerCollector) getContainerdImages() ([]ImageInfo, error) {
	if cc.ctrdClient == nil {
		return nil, fmt.Errorf("containerd client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ContainerAPITimeout)
	defer cancel()

	ctx = namespaces.WithNamespace(ctx, "default")

	images, err := cc.ctrdClient.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %v", err)
	}

	var result []ImageInfo
	for _, img := range images {
		name := img.Name()
		repo, tag := cc.parseRepoTag(name)

		size := "unknown"
		imgSize, err := img.Size(ctx)
		if err == nil {
			size = cc.formatSize(imgSize)
		}

		id := img.Target().Digest.String()
		if strings.HasPrefix(id, "sha256:") {
			id = id[7:]
		}
		if len(id) > 12 {
			id = id[:12]
		}

		result = append(result, ImageInfo{
			ID:         id,
			Repository: repo,
			Tag:        tag,
			Size:       size,
		})
	}

	return result, nil
}

// getCRIOContainers 使用 CRI API 获取容器
func (cc *ContainerCollector) getCRIOContainers() ([]ContainerInfo, error) {
	if cc.criClient == nil {
		return nil, fmt.Errorf("CRI client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ContainerAPITimeout)
	defer cancel()

	resp, err := cc.criClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	var result []ContainerInfo
	for _, c := range resp.Containers {
		id := c.Id
		if len(id) > 12 {
			id = id[:12]
		}

		name := ""
		if c.Metadata != nil {
			name = c.Metadata.Name
		}
		if name == "" {
			name = id
		}

		status := cc.criStateToString(c.State)
		created := time.Unix(0, c.CreatedAt).Format("2006-01-02 15:04:05")

		image := ""
		if c.Image != nil {
			image = c.Image.Image
		}

		result = append(result, ContainerInfo{
			ID:      id,
			Name:    name,
			Image:   image,
			Status:  status,
			Created: created,
		})
	}

	return result, nil
}

// getCRIOImages 使用 CRI API 获取镜像
func (cc *ContainerCollector) getCRIOImages() ([]ImageInfo, error) {
	if cc.criImageCli == nil {
		return nil, fmt.Errorf("CRI image client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ContainerAPITimeout)
	defer cancel()

	resp, err := cc.criImageCli.ListImages(ctx, &runtimeapi.ListImagesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %v", err)
	}

	var result []ImageInfo
	for _, img := range resp.Images {
		id := img.Id
		if strings.HasPrefix(id, "sha256:") {
			id = id[7:]
		}
		if len(id) > 12 {
			id = id[:12]
		}

		size := cc.formatSize(int64(img.Size))

		if len(img.RepoTags) == 0 {
			result = append(result, ImageInfo{
				ID:         id,
				Repository: "<none>",
				Tag:        "<none>",
				Size:       size,
			})
		} else {
			for _, tag := range img.RepoTags {
				repo, tagName := cc.parseRepoTag(tag)
				result = append(result, ImageInfo{
					ID:         id,
					Repository: repo,
					Tag:        tagName,
					Size:       size,
				})
			}
		}
	}

	return result, nil
}

// criStateToString 将 CRI 容器状态转换为字符串
func (cc *ContainerCollector) criStateToString(state runtimeapi.ContainerState) string {
	switch state {
	case runtimeapi.ContainerState_CONTAINER_CREATED:
		return "created"
	case runtimeapi.ContainerState_CONTAINER_RUNNING:
		return "running"
	case runtimeapi.ContainerState_CONTAINER_EXITED:
		return "exited"
	case runtimeapi.ContainerState_CONTAINER_UNKNOWN:
		return "unknown"
	default:
		return "unknown"
	}
}

// parseRepoTag 解析镜像标签
func (cc *ContainerCollector) parseRepoTag(repoTag string) (string, string) {
	if idx := strings.Index(repoTag, "@"); idx != -1 {
		return repoTag[:idx], "<digest>"
	}

	lastColon := strings.LastIndex(repoTag, ":")
	if lastColon == -1 {
		return repoTag, "latest"
	}

	afterColon := repoTag[lastColon+1:]
	if strings.Contains(afterColon, "/") {
		return repoTag, "latest"
	}

	return repoTag[:lastColon], afterColon
}

// formatSize 格式化大小
func (cc *ContainerCollector) formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2fGB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2fMB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2fKB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%dB", size)
	}
}
