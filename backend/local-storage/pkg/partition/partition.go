// Package partition wraps the system partitioning tools (lsblk,
// partx, parted, partprobe, blkid) and exposes a typed Partition
// type that joins the lsblk + partx properties for a single
// partition. Used by the disk-management service to drive the
// "Add new disk" UI flow.
package partition

import (
	"bytes"
	"errors"
	"strconv"
	"time"

	"github.com/neochaotic/powerlab/backend/local-storage/pkg/utils/command"
)

// Partition holds the joined lsblk + partx output for a single
// partition. Both maps use the upstream tool's lowercase property
// names (e.g. "name", "size", "uuid", "label").
type Partition struct {
	LSBLKProperties map[string]string
	PARTXProperties map[string]string
}

var ErrNoPartitionFound = errors.New("no partition found after partition creation")

// GetDevicePath resolves a filesystem UUID to a /dev/* path via
// blkid. Returns "" + error when blkid can't find the UUID.
func GetDevicePath(uuid string) (string, error) {
	out, err := command.ExecuteCommand("blkid", "--uuid", uuid)
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(out)), nil
}

// GetPartitions returns every partition on the given block device
// (path is e.g. /dev/sda) by running lsblk + partx and merging
// their per-partition property maps.
func GetPartitions(path string) ([]Partition, error) {
	var partitions []Partition

	// lsblk
	out, err := command.ExecuteCommand("lsblk", "--pairs", "--bytes", "--output-all", path)
	if err != nil {
		return nil, err
	}
	lsblkPartitions := parseLSBLKOutput(out)

	if len(lsblkPartitions) == 0 {
		return partitions, nil
	}

	// partx
	out, err = command.ExecuteCommand("partx", "--pairs", "--bytes", "--output-all", path)
	if err != nil {
		return nil, err
	}
	partxPartitions := parsePARTXOutput(out)

	if len(partxPartitions) == 0 {
		return partitions, nil
	}

	// merge
	partitions = mergeOutputs(lsblkPartitions, partxPartitions)

	return partitions, nil
}

// ProbePartition tells the kernel to re-read the partition table
// for device. Required after parted writes a new table — without
// this the new partitions don't appear in /dev/* until reboot.
func ProbePartition(device string) error {
	if _, err := command.ExecuteCommand("partprobe", "-s", device); err != nil {
		return err
	}

	return nil
}

// AddPartition partitions the given block device with a single
// primary partition spanning the full disk. Polls for up to 5
// seconds for the partition to appear, then returns it. Errors
// with ErrNoPartitionFound if the partition never materialises.
func AddPartition(rootDevice string) ([]Partition, error) {
	// add partition
	if _, err := command.ExecuteCommand("parted", "-s", rootDevice, "mkpart", "primary", "0", "100%"); err != nil {
		return nil, err
	}

	if err := ProbePartition(rootDevice); err != nil {
		return nil, err
	}

	var partitions []Partition
	count := 5
	for count > 0 {
		// wait for partition to appear
		result, err := GetPartitions(rootDevice)
		if err != nil {
			return nil, err
		}
		if len(result) > 0 {
			partitions = result
			break
		}

		time.Sleep(1 * time.Second)
		count--
	}

	if len(partitions) == 0 {
		return nil, ErrNoPartitionFound
	}

	return partitions, nil
}

// CreatePartitionTable writes a fresh GPT partition label to
// rootDevice. Destructive — wipes any existing partition table.
func CreatePartitionTable(rootDevice string) error {
	// create partition table
	if _, err := command.ExecuteCommand("parted", "-s", rootDevice, "mklabel", "gpt"); err != nil {
		return err
	}
	return nil
}

// partitionDevice - partition device, e.g. /dev/sda1
func FormatPartition(partitionDevice string) error {
	if _, err := command.ExecuteCommand(
		"mkfs.ext4",
		"-v",      // Verbose execution.
		"-m", "1", // Specify  the  percentage of the file system blocks reserved for the super-user.
		"-F",
		partitionDevice,
	); err != nil {
		return err
	}

	return nil
}

// rootDevice - root device, e.g. /dev/sda
//
// number - partition number, e.g. 1
func DeletePartition(rootDevice string, number int) error {
	n := strconv.Itoa(number)

	// delete partition
	if _, err := command.ExecuteCommand("sfdisk", "--delete", rootDevice, n); err != nil {
		return err
	}

	return ProbePartition(rootDevice)
}

func parsePARTXOutput(out []byte) map[string]map[string]string {
	partitions := map[string]map[string]string{}
	for _, buf := range bytes.Split(out, []byte("\n")) {
		if len(buf) == 0 {
			continue
		}

		partition := parsePairs(buf)
		if partition["UUID"] == "" {
			continue
		}

		partitions[partition["UUID"]] = partition
	}
	return partitions
}

func parseLSBLKOutput(out []byte) map[string]map[string]string {
	partitions := map[string]map[string]string{}
	for _, buf := range bytes.Split(out, []byte("\n")) {
		if len(buf) == 0 {
			continue
		}

		partition := parsePairs(buf)
		if partition["PARTUUID"] == "" {
			continue
		}

		partitions[partition["PARTUUID"]] = partition
	}
	return partitions
}

func mergeOutputs(lsblkPartitions, partxPartitions map[string]map[string]string) []Partition {
	partitions := []Partition{}
	for uuid, partxPartition := range partxPartitions {
		lsblkPartition, ok := lsblkPartitions[uuid]
		if !ok {
			continue
		}
		partitions = append(partitions, Partition{
			LSBLKProperties: lsblkPartition,
			PARTXProperties: partxPartition,
		})
	}

	return partitions
}

func parsePairs(buf []byte) map[string]string {
	pairs := map[string]string{}
	for _, field := range bytes.Fields(buf) {
		kv := bytes.Split(field, []byte("="))
		if len(kv) != 2 {
			continue
		}
		pairs[string(kv[0])] = string(bytes.Trim(kv[1], "\""))
	}

	return pairs
}
