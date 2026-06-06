// Package sysinfo provides system information tools for agents.
package sysinfo

import (
	"context"
	"encoding/json"
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// loadAvg wraps load.Avg for cross-platform compatibility
func loadAvg() (*load.AvgStat, error) {
	return load.Avg()
}

// Pack returns the system info tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("sysinfo").
		WithDescription("System information and monitoring tools").
		AddTools(
			hostInfoTool(),
			cpuInfoTool(),
			cpuUsageTool(),
			memoryInfoTool(),
			diskInfoTool(),
			diskUsageTool(),
			networkInfoTool(),
			networkIOTool(),
			uptimeTool(),
			loadAvgTool(),
			usersTool(),
			runtimeInfoTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func hostInfoTool() tool.Tool {
	return tool.NewBuilder("sysinfo_host").
		WithDescription("Get host system information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			info, err := host.InfoWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"hostname":         info.Hostname,
				"os":               info.OS,
				"platform":         info.Platform,
				"platform_family":  info.PlatformFamily,
				"platform_version": info.PlatformVersion,
				"kernel_version":   info.KernelVersion,
				"kernel_arch":      info.KernelArch,
				"uptime":           info.Uptime,
				"boot_time":        info.BootTime,
				"procs":            info.Procs,
				"host_id":          info.HostID,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cpuInfoTool() tool.Tool {
	return tool.NewBuilder("sysinfo_cpu").
		WithDescription("Get CPU information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			info, err := cpu.InfoWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			counts, _ := cpu.CountsWithContext(ctx, true)
			countLogical, _ := cpu.CountsWithContext(ctx, true)
			countPhysical, _ := cpu.CountsWithContext(ctx, false)

			var cpus []map[string]any
			for _, c := range info {
				cpus = append(cpus, map[string]any{
					"model":    c.ModelName,
					"vendor":   c.VendorID,
					"family":   c.Family,
					"cores":    c.Cores,
					"mhz":      c.Mhz,
					"cache_kb": c.CacheSize,
				})
			}

			result := map[string]any{
				"cpus":           cpus,
				"total_cores":    counts,
				"logical_cores":  countLogical,
				"physical_cores": countPhysical,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cpuUsageTool() tool.Tool {
	return tool.NewBuilder("sysinfo_cpu_usage").
		WithDescription("Get current CPU usage percentage").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PerCPU bool `json:"per_cpu,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			percent, err := cpu.PercentWithContext(ctx, 0, params.PerCPU)
			if err != nil {
				return tool.Result{}, err
			}

			if params.PerCPU {
				result := map[string]any{
					"per_cpu": percent,
					"count":   len(percent),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			total := 0.0
			if len(percent) > 0 {
				total = percent[0]
			}
			result := map[string]any{
				"percent": total,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func memoryInfoTool() tool.Tool {
	return tool.NewBuilder("sysinfo_memory").
		WithDescription("Get memory information").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			vmem, err := mem.VirtualMemoryWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			swap, _ := mem.SwapMemoryWithContext(ctx)

			result := map[string]any{
				"total":        vmem.Total,
				"available":    vmem.Available,
				"used":         vmem.Used,
				"free":         vmem.Free,
				"percent":      vmem.UsedPercent,
				"total_gb":     float64(vmem.Total) / 1024 / 1024 / 1024,
				"available_gb": float64(vmem.Available) / 1024 / 1024 / 1024,
				"used_gb":      float64(vmem.Used) / 1024 / 1024 / 1024,
			}

			if swap != nil {
				result["swap_total"] = swap.Total
				result["swap_used"] = swap.Used
				result["swap_free"] = swap.Free
				result["swap_percent"] = swap.UsedPercent
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func diskInfoTool() tool.Tool {
	return tool.NewBuilder("sysinfo_disk").
		WithDescription("Get disk partition information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				All bool `json:"all,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			partitions, err := disk.PartitionsWithContext(ctx, params.All)
			if err != nil {
				return tool.Result{}, err
			}

			var disks []map[string]any
			for _, p := range partitions {
				disks = append(disks, map[string]any{
					"device":     p.Device,
					"mountpoint": p.Mountpoint,
					"fstype":     p.Fstype,
					"opts":       p.Opts,
				})
			}

			result := map[string]any{
				"partitions": disks,
				"count":      len(disks),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func diskUsageTool() tool.Tool {
	return tool.NewBuilder("sysinfo_disk_usage").
		WithDescription("Get disk usage for a path").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			path := params.Path
			if path == "" {
				path = "/"
			}

			usage, err := disk.UsageWithContext(ctx, path)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"path":         path,
				"fstype":       usage.Fstype,
				"total":        usage.Total,
				"free":         usage.Free,
				"used":         usage.Used,
				"percent":      usage.UsedPercent,
				"total_gb":     float64(usage.Total) / 1024 / 1024 / 1024,
				"free_gb":      float64(usage.Free) / 1024 / 1024 / 1024,
				"used_gb":      float64(usage.Used) / 1024 / 1024 / 1024,
				"inodes_total": usage.InodesTotal,
				"inodes_used":  usage.InodesUsed,
				"inodes_free":  usage.InodesFree,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func networkInfoTool() tool.Tool {
	return tool.NewBuilder("sysinfo_network").
		WithDescription("Get network interface information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			interfaces, err := net.InterfacesWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			var ifaces []map[string]any
			for _, iface := range interfaces {
				var addrs []string
				for _, addr := range iface.Addrs {
					addrs = append(addrs, addr.Addr)
				}

				ifaces = append(ifaces, map[string]any{
					"name":      iface.Name,
					"mtu":       iface.MTU,
					"mac":       iface.HardwareAddr,
					"flags":     iface.Flags,
					"addresses": addrs,
				})
			}

			result := map[string]any{
				"interfaces": ifaces,
				"count":      len(ifaces),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func networkIOTool() tool.Tool {
	return tool.NewBuilder("sysinfo_network_io").
		WithDescription("Get network I/O statistics").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PerNic bool `json:"per_nic,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			counters, err := net.IOCountersWithContext(ctx, params.PerNic)
			if err != nil {
				return tool.Result{}, err
			}

			var stats []map[string]any
			for _, c := range counters {
				stats = append(stats, map[string]any{
					"name":         c.Name,
					"bytes_sent":   c.BytesSent,
					"bytes_recv":   c.BytesRecv,
					"packets_sent": c.PacketsSent,
					"packets_recv": c.PacketsRecv,
					"errin":        c.Errin,
					"errout":       c.Errout,
					"dropin":       c.Dropin,
					"dropout":      c.Dropout,
				})
			}

			result := map[string]any{
				"io_counters": stats,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func uptimeTool() tool.Tool {
	return tool.NewBuilder("sysinfo_uptime").
		WithDescription("Get system uptime").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			uptime, err := host.UptimeWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			days := uptime / 86400
			hours := (uptime % 86400) / 3600
			minutes := (uptime % 3600) / 60
			seconds := uptime % 60

			result := map[string]any{
				"seconds":   uptime,
				"days":      days,
				"hours":     hours,
				"minutes":   minutes,
				"remaining": seconds,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func loadAvgTool() tool.Tool {
	return tool.NewBuilder("sysinfo_load_avg").
		WithDescription("Get system load average").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			avg, err := loadAvg()
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"load1":  avg.Load1,
				"load5":  avg.Load5,
				"load15": avg.Load15,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func usersTool() tool.Tool {
	return tool.NewBuilder("sysinfo_users").
		WithDescription("Get logged in users").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			users, err := host.UsersWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			var userList []map[string]any
			for _, u := range users {
				userList = append(userList, map[string]any{
					"user":     u.User,
					"terminal": u.Terminal,
					"host":     u.Host,
					"started":  u.Started,
				})
			}

			result := map[string]any{
				"users": userList,
				"count": len(userList),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func runtimeInfoTool() tool.Tool {
	return tool.NewBuilder("sysinfo_runtime").
		WithDescription("Get Go runtime information").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			result := map[string]any{
				"go_version":     runtime.Version(),
				"go_os":          runtime.GOOS,
				"go_arch":        runtime.GOARCH,
				"num_cpu":        runtime.NumCPU(),
				"num_goroutine":  runtime.NumGoroutine(),
				"alloc_mb":       float64(m.Alloc) / 1024 / 1024,
				"total_alloc_mb": float64(m.TotalAlloc) / 1024 / 1024,
				"sys_mb":         float64(m.Sys) / 1024 / 1024,
				"num_gc":         m.NumGC,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
