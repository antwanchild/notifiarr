package snapshot

import (
	"bufio"
	"context"
	"fmt"
	"path"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v3/disk"
)

// ErrNoDisks is returned when no disks are found.
var ErrNoDisks = fmt.Errorf("no disks found")

func (s *Snapshot) getDriveData(ctx context.Context, run bool, useSudo bool) (errs []error) {
	if !run {
		s.Debug("Snapshot: skipping drive data")
		return nil
	}

	disks := make(map[string]string)

	err := s.getParts(ctx, disks)
	if err != nil {
		errs = append(errs, err)
	}

	s.Debug("Snapshot: got parts: %v", disks)

	if !mnd.IsDarwin {
		// We also do this because getParts doesn't always return (all the) disk drives.
		s.Debug("Snapshot: getting smart disks %v", disks)

		if err := s.getSmartDisks(ctx, useSudo, disks); err != nil {
			errs = append(errs, err)
		}

		s.Debug("Snapshot: getting blocks %v", disks)

		if err := s.getBlocks(disks); err != nil {
			errs = append(errs, err)
		}
	}

	s.Debug("Snapshot: disks %v", disks)

	if len(disks) == 0 {
		return append(errs, ErrNoDisks)
	}

	s.DriveAges = make(map[string]int)
	s.DriveTemps = make(map[string]int)
	s.DiskHealth = make(map[string]string)

	for name, dev := range disks {
		errs = append(errs, s.getDiskData(ctx, name, dev, useSudo))
	}

	return errs
}

func (s *Snapshot) getSmartDisks(ctx context.Context, useSudo bool, disks map[string]string) error {
	cmd, stdout, waitg, err := readyCommand(ctx, useSudo, "smartctl", "--scan-open")
	if err != nil {
		return err
	}

	go func() {
		for stdout.Scan() {
			fields := strings.Fields(stdout.Text())
			s.Debug("Snapshot: scan-open %v", fields)

			if len(fields) < 3 || fields[0] == "#" {
				continue
			}

			if strings.Contains(fields[2], ",") {
				disks[fields[2]] = fields[0]
			} else {
				disks[fields[0]] = fields[2]
			}
		}
		waitg.Done()
	}()

	return runCommand(cmd, waitg)
}

// use this for everything else....
func (s *Snapshot) getParts(ctx context.Context, disks map[string]string) error {
	const macDiskPrefix = "/dev/disk"

	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return fmt.Errorf("unable to get partitions: %w", err)
	}

	for _, part := range partitions {
		s.Debug("Snapshot: partition: %v", part)

		if mnd.IsDarwin {
			if !strings.HasPrefix(part.Device, macDiskPrefix) || slices.Contains(part.Opts, "nobrowse") {
				continue
			}

			stop := strings.Index(strings.TrimPrefix(part.Device, macDiskPrefix), "s")
			if stop > 0 {
				part.Device = part.Device[:stop+len(macDiskPrefix)]
			}
		}

		disks[part.Device] = ""
	}

	return nil
}

//nolint:cyclop
func (s *Snapshot) getDiskData(ctx context.Context, name, dev string, useSudo bool) error {
	args := []string{"-AH", name}

	switch {
	case strings.HasPrefix(name, "/dev/md") || strings.HasPrefix(name, "/dev/ram") ||
		strings.HasPrefix(name, "/dev/zram") || strings.HasPrefix(name, "/dev/synoboot") ||
		strings.HasPrefix(name, "/dev/nbd") || strings.HasPrefix(name, "/dev/vda"):
		return nil
	case mnd.IsSynology:
		args = []string{"-d", "sat", "-AH", name}
	case dev != "" && strings.Contains(name, ","):
		args = []string{"-d", name, "-AH", dev}
	case dev != "":
		args = []string{"-d", dev, "-AH", name}
	}

	cmd, stdout, waitg, err := readyCommand(ctx, useSudo, "smartctl", args...)
	if err != nil {
		return err
	}

	go s.scanSmartctl(stdout, name, waitg)

	return runCommand(cmd, waitg)
}

// scanSmartctl attempts to parse the varying outputs of smartctl disk health, age and temperature.
// Some disks seem to output in a completely different format than others, using the same tool.
//
//nolint:cyclop
func (s *Snapshot) scanSmartctl(stdout *bufio.Scanner, name string, waitg *sync.WaitGroup) {
	for stdout.Scan() {
		text := stdout.Text()

		switch fields := strings.Fields(text); {
		case strings.HasPrefix(text, "Current Drive Temperature:"):
			s.DriveTemps[name], _ = strconv.Atoi(fields[3])
		case strings.HasPrefix(text, "Accumulated power on time, hours:minutes"):
			s.DriveAges[name], _ = strconv.Atoi(strings.Split(fields[5], ":")[0])
		case len(fields) > 1 && fields[0] == "Temperature:":
			s.DriveTemps[name], _ = strconv.Atoi(fields[1])
		case len(fields) > 3 && fields[0]+fields[1]+fields[2] == "PowerOnHours:":
			s.DriveAges[name], _ = strconv.Atoi(strings.ReplaceAll(fields[3], ",", ""))
		case strings.Contains(text, "self-assessment ") ||
			strings.Contains(text, "SMART Health Status:"):
			s.DiskHealth[name] = fields[len(fields)-1]
		case len(fields) < 10: //nolint: gomnd
			continue
		case strings.HasPrefix(fields[1], "Airflow_Temp") ||
			strings.HasPrefix(fields[1], "Temperature_Cel"):
			s.DriveTemps[name], _ = strconv.Atoi(fields[9])
		case strings.HasPrefix(fields[1], "Power_On_Hour"):
			s.DriveAges[name], _ = strconv.Atoi(fields[9])
		}
	}
	waitg.Done()
}

func (s *Snapshot) getBlocks(disks map[string]string) error {
	block, err := ghw.Block()
	if err != nil {
		return fmt.Errorf("unable to get block devices: %w", err)
	}

	have := make(map[string]struct{})
	for _, dev := range block.Disks {
		s.Debug("Snapshot: block dev: %v", dev)

		if _, ok := have[dev.BusPath]; ok && strings.ToLower(dev.BusPath) != "unknown" {
			continue
		}

		have[dev.BusPath] = struct{}{}

		if runtime.GOOS != mnd.Windows {
			disks[path.Join("/dev", dev.Name)] = ""
		} else {
			disks[dev.Name] = ""
		}
	}

	return nil
}
