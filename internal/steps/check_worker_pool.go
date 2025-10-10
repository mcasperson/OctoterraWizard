package steps

import (
	"bytes"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/samber/lo"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/mcasperson/OctoterraWizard/internal/logutil"
	"github.com/mcasperson/OctoterraWizard/internal/octoclient"
	"github.com/mcasperson/OctoterraWizard/internal/strutil"
	"github.com/mcasperson/OctoterraWizard/internal/wizard"
)

//go:embed modules/worker_pool/terraform.tf
var workerPoolModule string

type CheckWorkerPoolStep struct {
	BaseStep
	Wizard           wizard.Wizard
	createWorkerPool *widget.Button
	next             *widget.Button
	previous         *widget.Button
	infinite         *widget.ProgressBarInfinite
	result           *widget.Label
	logs             *widget.Entry
	exportDone       bool
}

func (s CheckWorkerPoolStep) GetContainer(parent fyne.Window) *fyne.Container {

	heading := widget.NewLabel("Check Worker Pool")
	heading.TextStyle = fyne.TextStyle{Bold: true}

	label1 := widget.NewLabel(strutil.TrimMultilineWhitespace(`
		The destination space needs to have a worker pool matching the name of the default pool in the source space.
		When migrating from an on-prem Octopus instance, this is usually "Default Worker Pool".
		Cloud Octopus instances likely do not have a worker pool called "Default Worker Pool".
		We will create the default worker pool if it does not exit.
	`))

	bottom, previous, next := s.BuildNavigation(func() {
		s.Wizard.ShowWizardStep(ProjectExportStep{
			Wizard:   s.Wizard,
			BaseStep: BaseStep{State: s.State}})
	}, func() {
		moveNext := func(proceed bool) {
			if !proceed {
				return
			}

			s.Wizard.ShowWizardStep(StartSpaceExportStep{
				Wizard:   s.Wizard,
				BaseStep: BaseStep{State: s.State}})
		}
		if !s.exportDone {
			dialog.NewConfirm(
				"Do you want to skip this step?",
				"If you have run this step previously you can skip this step", moveNext, s.Wizard.Window).Show()
		} else {
			moveNext(true)
		}
	})
	s.next = next
	s.previous = previous

	s.infinite = widget.NewProgressBarInfinite()
	s.infinite.Start()
	s.infinite.Hide()
	s.result = widget.NewLabel("")
	s.logs = widget.NewEntry()
	s.logs.Disable()
	s.logs.MultiLine = true
	s.logs.SetMinRowsVisible(20)
	s.logs.Hide()

	s.createWorkerPool = widget.NewButton("Create Worker Pool", func() {
		s.exportDone = true
		s.createDefaultWorkerPool(parent)
	})

	middle := container.New(layout.NewVBoxLayout(), heading, label1, s.createWorkerPool, s.infinite, s.result, s.logs)

	content := container.NewBorder(nil, bottom, nil, nil, middle)

	return content
}

func (s CheckWorkerPoolStep) createDefaultWorkerPool(parent fyne.Window) {
	s.result.SetText("")
	s.logs.SetText("")
	s.next.Disable()
	s.previous.Disable()
	s.infinite.Show()
	s.logs.Hide()
	s.createWorkerPool.Disable()
	s.result.SetText("🔵 Creating worker pool.")

	go func() {
		s.Execute(
			// status
			func(message string) {
				fyne.Do(func() {
					s.result.SetText(message)
				})
			},
			// done
			func() {
				fyne.Do(func() {
					s.next.Enable()
					s.previous.Enable()
					s.infinite.Hide()
					s.createWorkerPool.Enable()
				})
			},
			// success
			func() {
				fyne.Do(func() {
					s.result.SetText("🟢 Added runbooks to all projects")
					s.logs.SetText("")
					s.logs.Hide()
				})
			},
			// error
			func(message string, err error) {
				fyne.Do(func() {
					if err := logutil.WriteTextToFile("check_worker_pool_error.txt", err.Error()); err != nil {
						fmt.Println("Failed to write error to file")
					}

					s.result.SetText(message)
					s.logs.SetText(err.Error())
					s.logs.Show()
					s.previous.Enable()
					s.next.Disable()
					s.infinite.Hide()
					s.createWorkerPool.Enable()
				})
			})
	}()
}

func (s CheckWorkerPoolStep) Execute(statusCallback func(message string), doneCallback func(), successCallback func(), errCallback func(string, error)) {
	defer doneCallback()

	sourceClient, err := octoclient.CreateClient(s.State)

	if err != nil {
		errCallback("🔴 Failed to create the source client", err)
		return
	}

	sourceWorkerPools, err := sourceClient.WorkerPools.GetAll()

	if err != nil {
		errCallback("🔴 Failed to create the source worker pools", err)
		return
	}

	defaultWorkerPool := lo.Filter(sourceWorkerPools, func(item *workerpools.WorkerPoolListResult, index int) bool {
		return item.IsDefault
	})

	if len(defaultWorkerPool) == 0 {
		statusCallback("🔵 There is no default worker pool on the source server, skipping creation.")
		return
	}

	destClient, err := octoclient.CreateDestinationClient(s.State)

	if err != nil {
		errCallback("🔴 Failed to create the destination client", err)
		return
	}

	destWorkerPools, err := destClient.WorkerPools.GetAll()

	if err != nil {
		errCallback("🔴 Failed to create the destination worker pools", err)
		return
	}

	matchingDestWorkerPool := lo.Filter(destWorkerPools, func(item *workerpools.WorkerPoolListResult, index int) bool {
		return item.Name == defaultWorkerPool[0].Name
	})

	if len(matchingDestWorkerPool) > 0 {
		statusCallback("🔵 The destination server already has a worker pool called " + defaultWorkerPool[0].Name + ", skipping creation.")
		return
	}

	// Save and apply the module
	dir, err := ioutil.TempDir("", "octoterra")
	if err != nil {
		errCallback("🔴 An error occurred while creating a temporary directory", err)
		return
	}

	filePath := filepath.Join(dir, "terraform.tf")
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			// ignore this and move on
			fmt.Println(err.Error())
		}
	}(filePath)

	if err := os.WriteFile(filePath, []byte(workerPoolModule), 0644); err != nil {
		errCallback("🔴 An error occurred while writing the Terraform file", err)
		return
	}

	initCmd := exec.Command("terraform", "init", "-no-color")
	initCmd.Dir = dir

	var initStdout, initStderr bytes.Buffer
	initCmd.Stdout = &initStdout
	initCmd.Stderr = &initStderr

	if err := initCmd.Run(); err != nil {
		errCallback("🔴 Terraform init failed.", err)
		return
	}

	applyCmd := exec.Command("terraform",
		"apply",
		"-auto-approve",
		"-no-color",
		"-var=octopus_server="+s.State.DestinationServer,
		"-var=octopus_apikey="+s.State.DestinationApiKey,
		"-var=octopus_space_id="+s.State.DestinationSpace,
		"-var=worker_pool_name="+defaultWorkerPool[0].Name)
	applyCmd.Dir = dir

	var stdout, stderr bytes.Buffer
	applyCmd.Stdout = &stdout
	applyCmd.Stderr = &stderr

	if err := applyCmd.Run(); err != nil {
		errCallback("🔴 Terraform apply failed", err)
		return
	} else {
		statusCallback("🔵 Terraform apply succeeded")
		fmt.Println(stdout.String() + stderr.String())
	}
}
