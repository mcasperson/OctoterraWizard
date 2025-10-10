package steps

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/mcasperson/OctoterraWizard/internal/logutil"
	"github.com/mcasperson/OctoterraWizard/internal/state"
	"github.com/mcasperson/OctoterraWizard/internal/validators"
	"github.com/mcasperson/OctoterraWizard/internal/wizard"
	"net/url"
	"strings"
)

type OctopusDetails struct {
	BaseStep
	Wizard   wizard.Wizard
	server   *widget.Entry
	apiKey   *widget.Entry
	spaceId  *widget.Entry
	result   *widget.Label
	next     *widget.Button
	previous *widget.Button
}

func (s OctopusDetails) GetContainer(parent fyne.Window) *fyne.Container {

	bottom, previous, next := s.BuildNavigation(func() {
		s.Wizard.ShowWizardStep(TestTerraformStep{
			Wizard:   s.Wizard,
			BaseStep: BaseStep{State: s.getState()}})
	}, func() {
		s.result.SetText("🔵 Validating Octopus credentials.")
		s.apiKey.Disable()
		s.server.Disable()
		s.spaceId.Disable()
		s.next.Disable()
		s.previous.Disable()
		defer s.apiKey.Enable()
		defer s.server.Enable()
		defer s.spaceId.Enable()
		defer s.next.Enable()
		defer s.previous.Enable()

		validationFailed := false
		if err := validators.ValidateSourceCreds(s.getState()); err != nil {
			if err := logutil.WriteTextToFile("octopus_details_error.txt", err.Error()); err != nil {
				fmt.Println("Failed to write error to file")
			}

			s.result.SetText("🔴 Unable to connect to the Octopus server. Please check the URL, API key, and Space ID.")
			validationFailed = true
		}

		nexCallback := func(proceed bool) {
			if proceed {
				s.Wizard.ShowWizardStep(OctopusDestinationDetails{
					Wizard:   s.Wizard,
					BaseStep: BaseStep{State: s.getState()}})
			}
		}

		if validationFailed {
			dialog.NewConfirm("Octopus Validation failed", "Validation of the Octopus details failed. Do you wish to continue anyway?", nexCallback, s.Wizard.Window).Show()
		} else {
			nexCallback(true)
		}
	})
	s.next = next
	s.previous = previous
	s.result = widget.NewLabel("")

	validation := func(input string) {
		next.Disable()

		if s.server == nil || s.server.Text == "" || s.apiKey == nil || s.apiKey.Text == "" || s.spaceId == nil || s.spaceId.Text == "" {
			return
		}

		next.Enable()
	}

	heading := widget.NewLabel("Octopus Source Server")
	heading.TextStyle = fyne.TextStyle{Bold: true}

	introText := widget.NewLabel("Enter the URL, API key, and Space ID of the Octopus instance you want to export from (i.e. the source server).")
	linkUrl, _ := url.Parse("https://octopus.com/docs/octopus-rest-api/how-to-create-an-api-key")
	link := widget.NewHyperlink("Learn how to create an API key.", linkUrl)

	serverLabel := widget.NewLabel("Source Server URL")
	s.server = widget.NewEntry()
	s.server.SetPlaceHolder("https://octopus.example.com")
	s.server.SetText(s.State.Server)

	apiKeyLabel := widget.NewLabel("Source API Key")
	s.apiKey = widget.NewPasswordEntry()
	s.apiKey.SetPlaceHolder("API-xxxxxxxxxxxxxxxxxxxxxxxxxx")
	s.apiKey.SetText(s.State.ApiKey)

	spaceIdLabel := widget.NewLabel("Source Space ID")
	s.spaceId = widget.NewEntry()
	s.spaceId.SetPlaceHolder("Spaces-#")
	s.spaceId.SetText(s.State.Space)

	validation("")

	s.server.OnChanged = validation
	s.apiKey.OnChanged = validation
	s.spaceId.OnChanged = validation

	formLayout := container.New(layout.NewFormLayout(), serverLabel, s.server, apiKeyLabel, s.apiKey, spaceIdLabel, s.spaceId)

	middle := container.New(layout.NewVBoxLayout(), heading, introText, link, formLayout, s.result)

	content := container.NewBorder(nil, bottom, nil, nil, middle)

	return content
}

func (s OctopusDetails) getState() state.State {
	return state.State{
		BackendType:               s.State.BackendType,
		Server:                    strings.TrimSpace(s.server.Text),
		ServerExternal:            s.State.ServerExternal,
		ApiKey:                    strings.TrimSpace(s.apiKey.Text),
		Space:                     strings.TrimSpace(s.spaceId.Text),
		DestinationServer:         s.State.DestinationServer,
		DestinationServerExternal: s.State.DestinationServerExternal,
		DestinationApiKey:         s.State.DestinationApiKey,
		DestinationSpace:          s.State.DestinationSpace,
		AwsAccessKey:              s.State.AwsAccessKey,
		AwsSecretKey:              s.State.AwsSecretKey,
		AwsS3Bucket:               s.State.AwsS3Bucket,
		AwsS3BucketRegion:         s.State.AwsS3BucketRegion,
		PromptForDelete:           s.State.PromptForDelete,
		UseContainerImages:        s.State.UseContainerImages,
		AzureResourceGroupName:    s.State.AzureResourceGroupName,
		AzureStorageAccountName:   s.State.AzureStorageAccountName,
		AzureContainerName:        s.State.AzureContainerName,
		AzureSubscriptionId:       s.State.AzureSubscriptionId,
		AzureTenantId:             s.State.AzureTenantId,
		AzureApplicationId:        s.State.AzureApplicationId,
		AzurePassword:             s.State.AzurePassword,
		DatabaseServer:            s.State.DatabaseServer,
		DatabaseUser:              s.State.DatabaseUser,
		DatabasePass:              s.State.DatabasePass,
		DatabasePort:              s.State.DatabasePort,
		DatabaseName:              s.State.DatabaseName,
		DatabaseMasterKey:         s.State.DatabaseMasterKey,
		EnableProjectRenaming:     s.State.EnableProjectRenaming,
	}
}
