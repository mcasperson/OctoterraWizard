package steps

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/mcasperson/OctoterraWizard/internal/infrastructure"
	"github.com/mcasperson/OctoterraWizard/internal/logutil"
	"github.com/mcasperson/OctoterraWizard/internal/octoclient"
	"github.com/mcasperson/OctoterraWizard/internal/query"
	"github.com/mcasperson/OctoterraWizard/internal/sensitivevariables"
	"github.com/mcasperson/OctoterraWizard/internal/strutil"
	"github.com/mcasperson/OctoterraWizard/internal/wizard"
)

//go:embed modules/project_management/terraform.tf
var runbookModule string

type ProjectExportStep struct {
	BaseStep
	Wizard        wizard.Wizard
	createProject *widget.Button
	infinite      *widget.ProgressBarInfinite
	result        *widget.Label
	logs          *widget.Entry
	next          *widget.Button
	previous      *widget.Button
	exportDone    bool
}

func (s ProjectExportStep) GetContainer(parent fyne.Window) *fyne.Container {

	bottom, thisPrevious, thisNext := s.BuildNavigation(func() {
		s.Wizard.ShowWizardStep(SpaceExportStep{
			Wizard:   s.Wizard,
			BaseStep: BaseStep{State: s.State}})
	}, func() {
		moveNext := func(proceed bool) {
			if !proceed {
				return
			}

			s.Wizard.ShowWizardStep(CheckWorkerPoolStep{
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
	s.next = thisNext
	s.previous = thisPrevious
	s.exportDone = false

	heading := widget.NewLabel("Project Serialization Runbooks")
	heading.TextStyle = fyne.TextStyle{Bold: true}

	intro := widget.NewLabel(strutil.TrimMultilineWhitespace(`Each project gets two runbooks: one to serialize it to a Terraform module, and the second to deploy it.`))
	s.infinite = widget.NewProgressBarInfinite()
	s.infinite.Start()
	s.infinite.Hide()
	s.result = widget.NewLabel("")
	s.logs = widget.NewEntry()
	s.logs.Disable()
	s.logs.MultiLine = true
	s.logs.SetMinRowsVisible(20)
	s.logs.Hide()
	s.createProject = widget.NewButton("Add Runbooks", func() {
		s.exportDone = true
		s.createNewProject(parent)
	})
	middle := container.New(layout.NewVBoxLayout(), heading, intro, s.createProject, s.infinite, s.result, s.logs)

	content := container.NewBorder(nil, bottom, nil, nil, middle)

	return content
}

func (s ProjectExportStep) createNewProject(parent fyne.Window) {
	s.result.SetText("")
	s.logs.SetText("")
	s.next.Disable()
	s.previous.Disable()
	s.infinite.Show()
	s.logs.Hide()
	s.createProject.Disable()
	s.result.SetText("🔵 Creating runbooks. This can take a little while.")

	go func() {
		s.Execute(
			// prompt
			func(title string, message string, callback func(bool)) {
				dialog.NewConfirm(title, message, callback, parent).Show()
			},
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
					s.createProject.Enable()
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
					if err := logutil.WriteTextToFile("project_export_error.txt", err.Error()); err != nil {
						fmt.Println("Failed to write error to file")
					}

					s.result.SetText(message)
					s.logs.SetText(err.Error())
					s.logs.Show()
					s.previous.Enable()
					s.next.Disable()
					s.infinite.Hide()
					s.createProject.Enable()
				})
			})
	}()
}

func (s ProjectExportStep) Execute(prompt func(string, string, func(bool)), statusCallback func(message string), doneCallback func(), successCallback func(), errCallback func(string, error)) {

	defer doneCallback()

	myclient, err := octoclient.CreateClient(s.State)

	if err != nil {
		errCallback("🔴 Failed to create the client", err)
		return
	}

	allProjects, err := infrastructure.GetProjects(myclient)

	if err != nil {
		errCallback("🔴 Failed to get all the projects", err)
		return
	}

	lvsExists, lvs, err := query.LibraryVariableSetExists(myclient, "Octoterra")

	if err != nil {
		errCallback("🔴 Failed to get the library variable set Octoterra", err)
		return
	}

	if !lvsExists {
		errCallback("🔴 The library variable set Octoterra could not be found. Make sure you have run the \"Space Serialization Runbooks\" step.", err)
		return
	}

	varsLvsExists, varsLvs, err := query.LibraryVariableSetExists(myclient, sensitivevariables.SecretsLibraryVariableSetName)

	if err != nil {
		errCallback("🔴 Failed to get the library variable set "+sensitivevariables.SecretsLibraryVariableSetName, err)
		return
	}

	// First look deletes any existing projects
	for _, project := range allProjects {
		if project.Name == spaceManagementProject {
			continue
		}

		runbookExists, runbook, err := s.runbookExists(myclient, project.ID, "__ 1. Serialize Project")

		if err != nil {
			errCallback("🔴 Failed to find runbook", err)
			return
		}

		if runbookExists {
			deleteRunbook1Func := func(b bool) {
				if b {
					if err := s.deleteRunbook(myclient, runbook); err != nil {
						errCallback("🔴 Failed to delete the resource", err)
					} else if s.State.PromptForDelete {
						s.Execute(prompt, statusCallback, doneCallback, successCallback, errCallback)
					}
				}
			}

			if s.State.PromptForDelete {
				prompt("Project Group Exists", "The runbook \""+runbook.Name+"\" already exists in project "+project.Name+". Do you want to delete it? It is usually safe to delete this resource.", deleteRunbook1Func)
				return
			} else {
				deleteRunbook1Func(true)
			}
		}

		runbook2Exists, runbook2, err := s.runbookExists(myclient, project.ID, "__ 2. Deploy Project")

		if err != nil {
			errCallback("🔴 Failed to find runbook", err)
			return
		}

		if runbook2Exists {
			deleteRunbook2Func := func(b bool) {
				if b {
					if err := s.deleteRunbook(myclient, runbook2); err != nil {
						errCallback("🔴 Failed to delete the resource", err)
					} else if s.State.PromptForDelete {
						s.Execute(prompt, statusCallback, doneCallback, successCallback, errCallback)
					}
				}
			}

			if s.State.PromptForDelete {
				prompt("Runbook Exists", "The runbook \""+runbook2.Name+"\" already exists in project "+project.Name+". Do you want to delete it? It is usually safe to delete this resource.", deleteRunbook2Func)
				return
			} else {
				deleteRunbook2Func(true)
			}
		}

		variableExists, matchingVariables, err := s.projectVariableExists(myclient, project.ID, "OctoterraWiz.Destination.ProjectName")

		if variableExists {
			for _, variable := range matchingVariables {
				deleteVariableFunc := func(b bool) {
					if b {
						if err := s.deleteProjectVariable(myclient, project.ID, variable); err != nil {
							errCallback("🔴 Failed to delete the resource", err)
						} else if s.State.PromptForDelete {
							s.Execute(prompt, statusCallback, doneCallback, successCallback, errCallback)
						}
					}
				}

				if s.State.PromptForDelete {
					prompt("Variable Exists", "The variable \""+variable.Name+"\" already exists in project "+project.Name+". Do you want to delete it? It is usually safe to delete this resource.", deleteVariableFunc)
					return
				} else {
					deleteVariableFunc(true)
				}
			}
		}
	}

	// Find the step template ID
	serializeProjectTemplate, err, message := query.GetStepTemplateId(myclient, s.State, "Octopus - Serialize Project to Terraform")

	if err != nil {
		errCallback(message, err)
		return
	}

	deploySpaceTemplateS3, err, message := query.GetStepTemplateId(myclient, s.State, "Octopus - Populate Octoterra Space (S3 Backend)")

	if err != nil {
		errCallback(message, err)
		return
	}

	deploySpaceTemplateAzureStorage, err, message := query.GetStepTemplateId(myclient, s.State, "Octopus - Populate Octoterra Space (Azure Backend)")

	if err != nil {
		errCallback(message, err)
	}

	for index, project := range allProjects {
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

		if err := os.WriteFile(filePath, []byte(runbookModule), 0644); err != nil {
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
			"-var=octopus_serialize_actiontemplateid="+serializeProjectTemplate,
			"-var=octopus_deploys3_actiontemplateid="+deploySpaceTemplateS3,
			"-var=octopus_deployazure_actiontemplateid="+deploySpaceTemplateAzureStorage,
			"-var=octopus_server_external="+s.State.GetExternalServer(),
			"-var=terraform_backend="+s.State.BackendType,
			"-var=use_container_images="+fmt.Sprint(s.State.UseContainerImages),
			"-var=default_secret_variables=false",
			"-var=customise_destination_project_name="+fmt.Sprint(s.State.EnableProjectRenaming),
			"-var=octopus_server="+s.State.Server,
			"-var=octopus_apikey="+s.State.ApiKey,
			"-var=octopus_space_id="+s.State.Space,
			"-var=octopus_project_id="+project.ID,
			"-var=terraform_state_bucket="+s.State.AwsS3Bucket,
			"-var=terraform_state_bucket_region="+s.State.AwsS3BucketRegion,
			"-var=terraform_state_azure_resource_group="+s.State.AzureResourceGroupName,
			"-var=terraform_state_azure_storage_account="+s.State.AzureStorageAccountName,
			"-var=terraform_state_azure_storage_container="+s.State.AzureContainerName,
			"-var=octopus_destination_server="+s.State.DestinationServer,
			"-var=octopus_destination_apikey="+s.State.DestinationApiKey,
			"-var=octopus_destination_space_id="+s.State.DestinationSpace,
			"-var=octopus_project_name="+project.Name)
		applyCmd.Dir = dir

		var stdout, stderr bytes.Buffer
		applyCmd.Stdout = &stdout
		applyCmd.Stderr = &stderr

		if err := applyCmd.Run(); err != nil {
			errCallback("🔴 Terraform apply failed", err)
			return
		} else {
			statusCallback("🔵 Terraform apply succeeded (" + fmt.Sprint(index) + " / " + fmt.Sprint(len(allProjects)) + ")")
			fmt.Println(stdout.String() + stderr.String())
		}

		// link the library variable set
		projectResource, err := myclient.Projects.GetByID(project.ID)

		if err != nil {
			errCallback("🔴 Failed to get the project", err)
			return
		}

		projectResource.IncludedLibraryVariableSets = append(projectResource.IncludedLibraryVariableSets, lvs.ID)

		// The secrets library variable set is optional
		if varsLvsExists {
			projectResource.IncludedLibraryVariableSets = append(projectResource.IncludedLibraryVariableSets, varsLvs.ID)
		}

		_, err = projects.Update(myclient, projectResource)

		if err != nil {
			errCallback("🔴 Failed to update the project", errors.New(err.Error()+" "+projectResource.ID+" "+projectResource.Name))
			return
		}

	}

	successCallback()
}

func (s ProjectExportStep) deleteRunbook(myclient *client.Client, runbook *runbooks.Runbook) error {
	fmt.Println("Attempting to delete runbook " + runbook.ID)
	if err := myclient.Runbooks.DeleteByID(runbook.ID); err != nil {
		return errors.Join(errors.New("failed to delete runbook with ID "+runbook.ID+" and name "+runbook.Name), err)
	}

	return nil
}

func (s ProjectExportStep) runbookExists(myclient *client.Client, projectId string, runbookName string) (bool, *runbooks.Runbook, error) {
	if runbook, err := runbooks.GetByName(myclient, myclient.GetSpaceID(), projectId, runbookName); err == nil {
		if runbook == nil {
			return false, nil, nil
		}
		return true, runbook, nil
	} else {
		return false, nil, errors.Join(errors.New("failed to get runbook by name "+runbookName+" in project "+projectId), err)
	}
}

func (s ProjectExportStep) deleteProjectVariable(myclient *client.Client, projectId string, variable *variables.Variable) error {
	fmt.Println("Attempting to delete variable " + variable.ID)
	if _, err := variables.DeleteSingle(myclient, myclient.GetSpaceID(), projectId, variable.ID); err != nil {
		return errors.Join(errors.New("failed to delete variable with ID "+variable.ID+" and name "+variable.Name), err)
	}

	return nil
}

func (s ProjectExportStep) projectVariableExists(myclient *client.Client, projectId string, variableName string) (bool, []*variables.Variable, error) {
	if variable, err := variables.GetByName(myclient, myclient.GetSpaceID(), projectId, variableName, &variables.VariableScope{}); err == nil {
		if variable == nil {
			return false, nil, nil
		}
		return true, variable, nil
	} else {
		return false, nil, errors.Join(errors.New("failed to get variable by name "+variableName+" in project "+projectId), err)
	}
}
