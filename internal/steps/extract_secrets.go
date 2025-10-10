package steps

import (
	"fmt"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/mcasperson/OctoterraWizard/internal/logutil"
	"github.com/mcasperson/OctoterraWizard/internal/sensitivevariables"
	"github.com/mcasperson/OctoterraWizard/internal/state"
	"github.com/mcasperson/OctoterraWizard/internal/strutil"
	"github.com/mcasperson/OctoterraWizard/internal/validators"
	"github.com/mcasperson/OctoterraWizard/internal/wizard"
)

// ExtractSecrets provides a step in the wizard to extract secrets from the Octopus database
type ExtractSecrets struct {
	BaseStep
	Wizard           wizard.Wizard
	dbServer         *widget.Entry
	port             *widget.Entry
	database         *widget.Entry
	username         *widget.Entry
	password         *widget.Entry
	masterKey        *widget.Entry
	result           *widget.Label
	next             *widget.Button
	previous         *widget.Button
	extractVariables *widget.Button
	extractDone      bool
}

func (s ExtractSecrets) GetContainer(parent fyne.Window) *fyne.Container {

	bottom, previous, next := s.BuildNavigation(func() {
		s.Wizard.ShowWizardStep(BackendSelectionStep{
			Wizard:   s.Wizard,
			BaseStep: BaseStep{State: s.State}})
	}, func() {
		s.dbServer.Disable()
		s.port.Disable()
		s.database.Disable()
		s.masterKey.Disable()
		s.next.Disable()
		s.previous.Disable()
		defer s.dbServer.Enable()
		defer s.port.Enable()
		defer s.database.Enable()
		defer s.masterKey.Enable()
		defer s.next.Enable()
		defer s.previous.Enable()

		nexCallback := func(proceed bool) {
			if proceed {
				s.Wizard.ShowWizardStep(StepTemplateStep{
					Wizard:   s.Wizard,
					BaseStep: BaseStep{State: s.State}})
			}
		}

		if !s.extractDone {
			dialog.NewConfirm(
				"Do you want to skip this step?",
				"If you have run this step previously you can skip this step.", nexCallback, s.Wizard.Window).Show()
		} else {
			nexCallback(true)
		}
	})
	s.next = next
	s.previous = previous
	s.result = widget.NewLabel("")

	validation := func(input string) {
		s.extractVariables.Disable()

		if s.dbServer.Text == "" || s.database.Text == "" || s.port.Text == "" || s.masterKey.Text == "" || s.password.Text == "" || s.username.Text == "" {
			return
		}

		s.extractVariables.Enable()
	}

	heading := widget.NewLabel("Sensitive Value Extraction")
	heading.TextStyle = fyne.TextStyle{Bold: true}

	introText := widget.NewLabel(strutil.TrimMultilineWhitespace(`
		Enter the Octopus database server, port, database name, username, password, and master key.
		The master key is used to decrypt the sensitive values stored in the database.
		Skip this step if you are migrating from a cloud instance, as you do not have database access.`))
	linkUrl, _ := url.Parse("https://octopus.com/docs/security/data-encryption")
	link := widget.NewHyperlink("Learn about the master key.", linkUrl)

	infinite := widget.NewProgressBarInfinite()
	infinite.Start()
	infinite.Hide()

	serverLabel := widget.NewLabel("Octopus Database Server")
	s.dbServer = widget.NewEntry()
	s.dbServer.SetPlaceHolder("192.168.1.1")
	s.dbServer.SetText(s.State.DatabaseServer)

	portLabel := widget.NewLabel("Octopus Database Port")
	s.port = widget.NewEntry()
	s.port.SetPlaceHolder("1433")
	s.port.SetText(fmt.Sprint(s.State.DatabasePort))

	databaseLabel := widget.NewLabel("Octopus Database Name")
	s.database = widget.NewEntry()
	s.database.SetPlaceHolder("Octopus")
	s.database.SetText(fmt.Sprint(s.State.DatabaseName))

	usernameLabel := widget.NewLabel("Octopus Database Username")
	s.username = widget.NewEntry()
	s.username.SetPlaceHolder("SA")
	s.username.SetText(fmt.Sprint(s.State.DatabaseUser))

	passwordLabel := widget.NewLabel("Octopus Database Password")
	s.password = widget.NewPasswordEntry()
	s.password.SetPlaceHolder("xxxxxxxxxxxxxxxxxxxxxxxxxx")
	s.password.SetText(s.State.DatabasePass)

	masterKeyPassword := widget.NewLabel("Octopus Database MasterKey")
	s.masterKey = widget.NewPasswordEntry()
	s.masterKey.SetPlaceHolder("xxxxxxxxxxxxxxxxxxxxxxxxxx")
	s.masterKey.SetText(s.State.DatabaseMasterKey)

	s.extractVariables = widget.NewButton("Extract sensitive values", func() {
		next.Disable()
		previous.Disable()
		infinite.Show()
		s.dbServer.Disable()
		s.password.Disable()
		s.username.Disable()
		s.database.Disable()
		s.port.Disable()
		s.masterKey.Disable()
		s.extractVariables.Disable()
		s.result.SetText("🔵 Extracting sensitive values.")
		s.extractDone = true

		go func() {
			s.Execute(
				func() {
					fyne.Do(func() {
						previous.Enable()
						next.Enable()
						s.dbServer.Enable()
						s.password.Enable()
						s.username.Enable()
						s.database.Enable()
						s.port.Enable()
						s.masterKey.Enable()
						s.extractVariables.Enable()
						infinite.Hide()
					})
				},
				func() {
					fyne.Do(func() {
						s.result.SetText("🟢 Sensitive values have been extracted.")
					})
				},
				func(err error) {
					if err := logutil.WriteTextToFile("extract_secrets_error.txt", err.Error()); err != nil {
						fmt.Println("Failed to write error to file")
					}

					fyne.Do(func() {
						s.result.SetText("🔴 An error was raised while attempting to extract the sensitive values." + err.Error())
					})
				})
		}()
	})

	validation("")

	s.dbServer.OnChanged = validation
	s.port.OnChanged = validation
	s.database.OnChanged = validation
	s.username.OnChanged = validation
	s.password.OnChanged = validation
	s.masterKey.OnChanged = validation

	formLayout := container.New(layout.NewFormLayout(), serverLabel, s.dbServer, portLabel, s.port, databaseLabel, s.database, usernameLabel, s.username, passwordLabel, s.password, masterKeyPassword, s.masterKey)

	middle := container.New(layout.NewVBoxLayout(), heading, introText, link, formLayout, s.extractVariables, infinite, s.result)

	content := container.NewBorder(nil, bottom, nil, nil, middle)

	return content
}

func (s ExtractSecrets) getState() state.State {
	return state.State{
		BackendType:                   s.State.BackendType,
		Server:                        s.State.Server,
		ServerExternal:                s.State.ServerExternal,
		ApiKey:                        s.State.ApiKey,
		Space:                         s.State.Space,
		DestinationServer:             s.State.DestinationServer,
		DestinationServerExternal:     s.State.DestinationServerExternal,
		DestinationApiKey:             s.State.DestinationApiKey,
		DestinationSpace:              s.State.DestinationSpace,
		AwsAccessKey:                  s.State.AwsAccessKey,
		AwsSecretKey:                  s.State.AwsSecretKey,
		AwsS3Bucket:                   s.State.AwsS3Bucket,
		AwsS3BucketRegion:             s.State.AwsS3BucketRegion,
		PromptForDelete:               s.State.PromptForDelete,
		UseContainerImages:            s.State.UseContainerImages,
		AzureResourceGroupName:        s.State.AzureResourceGroupName,
		AzureStorageAccountName:       s.State.AzureStorageAccountName,
		AzureContainerName:            s.State.AzureContainerName,
		AzureSubscriptionId:           s.State.AzureSubscriptionId,
		AzureTenantId:                 s.State.AzureTenantId,
		AzureApplicationId:            s.State.AzureApplicationId,
		AzurePassword:                 s.State.AzurePassword,
		ExcludeAllLibraryVariableSets: false,
		EnableVariableSpreading:       false,
		DatabaseServer:                strings.TrimSpace(s.dbServer.Text),
		DatabaseUser:                  strings.TrimSpace(s.username.Text),
		DatabasePass:                  strings.TrimSpace(s.password.Text),
		DatabasePort:                  strings.TrimSpace(s.port.Text),
		DatabaseName:                  strings.TrimSpace(s.database.Text),
		DatabaseMasterKey:             strings.TrimSpace(s.masterKey.Text),
		EnableProjectRenaming:         s.State.EnableProjectRenaming,
	}
}

// SaveSecretsVariable creates a library variable set with a secret value containing the contents
// of a terraform.tfvars file that populates the secrets used by the exported space
func (s *ExtractSecrets) SaveSecretsVariable() error {
	return nil
}

func (s ExtractSecrets) Execute(doneCallback func(), successCallback func(), errCallback func(error)) {
	defer doneCallback()

	if err := validators.ValidateDatabase(s.getState()); err != nil {
		if err := logutil.WriteTextToFile("extract_secrets_error.txt", err.Error()); err != nil {
			errCallback(err)
			return
		}
	}

	newState := s.getState()
	variableValue, err := sensitivevariables.ExtractVariables(newState.DatabaseServer, newState.DatabasePort, newState.DatabaseName, newState.DatabaseUser, newState.DatabasePass, newState.DatabaseMasterKey)

	if err != nil {
		if err := logutil.WriteTextToFile("extract_secrets_error.txt", err.Error()); err != nil {
			fmt.Println("Failed to write error to file")
		}
		errCallback(err)
		return
	} else {
		if err := sensitivevariables.CreateSecretsLibraryVariableSet(variableValue, newState); err != nil {
			if err := logutil.WriteTextToFile("extract_secrets_error.txt", err.Error()); err != nil {
				fmt.Println("Failed to write error to file")
			}

			errCallback(err)
			return
		}
	}

	successCallback()
}
