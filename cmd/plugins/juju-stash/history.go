// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/juju/errors"

	"github.com/juju/juju/juju/osenv"
)

func newHistory() (*history, error) {
	h := &history{
		stashPath: filepath.Join(osenv.JujuXDGDataHomeDir(), "stash.log"),
	}
	return h, h.read()
}

// history holds all the changes for a model stash
type history struct {
	stashPath string
	log       []historySnapshot
}

type historySnapshot struct {
	controllerName string
	modelName      string
}

// Push a model snapshot onto the history log
func (h *history) Push(name historySnapshot) error {
	h.log = append(h.log, name)
	return h.write()
}

// Pop a model snapshot from the history log
func (h *history) Pop() (historySnapshot, error) {
	if err := h.read(); err != nil {
		return historySnapshot{}, errors.Trace(err)
	}
	if len(h.log) < 1 {
		return historySnapshot{}, errors.New("nothing to pop")
	}
	snapshot := h.log[len(h.log)-1]
	h.log = h.log[:len(h.log)-1]
	if err := h.write(); err != nil {
		return historySnapshot{}, errors.Trace(err)
	}
	return snapshot, nil
}

func (h *history) Snapshots() ([]historySnapshot, error) {
	if err := h.read(); err != nil {
		return nil, errors.Trace(err)
	}
	return h.log, nil
}

func (h *history) read() error {
	if _, err := os.Stat(h.stashPath); os.IsNotExist(err) {
		file, err := os.Create(h.stashPath)
		if err != nil {
			return errors.Trace(err)
		}
		file.Close()
	}

	file, err := os.Open(h.stashPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	h.log = make([]historySnapshot, 0)
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		h.log = append(h.log, historySnapshot{
			controllerName: parts[0],
			modelName:      parts[1],
		})
	}
	return nil
}

func (h *history) write() error {
	newFileName := fmt.Sprintf("%s.new", h.stashPath)
	file, err := os.Create(newFileName)
	if err != nil {
		return errors.Trace(err)
	}
	defer file.Close()

	for _, snapshot := range h.log {
		if _, err := file.WriteString(fmt.Sprintf("%s %s\n", snapshot.controllerName, snapshot.modelName)); err != nil {
			return errors.Trace(err)
		}
	}

	if err := file.Sync(); err != nil {
		return errors.Trace(err)
	}
	if err := file.Close(); err != nil {
		return errors.Trace(err)
	}

	if err := os.Remove(h.stashPath); err != nil {
		return errors.Trace(err)
	}
	return os.Rename(newFileName, h.stashPath)
}
