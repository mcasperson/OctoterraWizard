package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/mcasperson/OctoterraWizard/internal/state"
	"github.com/mcasperson/OctoterraWizard/internal/steps"
	"github.com/mcasperson/OctoterraWizard/internal/wizard"
	"image/color"
	"os"
	"strings"
)

var Version = "development"

type myTheme struct{}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (m myTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m myTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func (m myTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func main() {

	wiz := wizard.NewWizard("Octoterra Wizard (" + Version + ")")
	wiz.App.Settings().SetTheme(&myTheme{})

	defaultSourceServer := os.Getenv("OCTOTERRAWIZ_SOURCE_SERVER")
	if defaultSourceServer == "" {
		defaultSourceServer = os.Getenv("OCTOPUS_CLI_SERVER")
	}

	defaultSourceServerApi := os.Getenv("OCTOTERRAWIZ_SOURCE_API_KEY")
	if defaultSourceServerApi == "" {
		defaultSourceServerApi = os.Getenv("OCTOPUS_CLI_API_KEY")
	}

	defaultSourceServerSpace := os.Getenv("OCTOTERRAWIZ_SOURCE_SPACE_ID")
	if defaultSourceServerSpace == "" {
		defaultSourceServerSpace = "Spaces-1"
	}

	defaultDestinationServer := os.Getenv("OCTOTERRAWIZ_DESTINATION_SERVER")
	if defaultDestinationServer == "" {
		defaultDestinationServer = os.Getenv("OCTOPUS_CLI_SERVER")
	}

	defaultDestinationServerApi := os.Getenv("OCTOTERRAWIZ_DESTINATION_API_KEY")
	if defaultDestinationServerApi == "" {
		defaultDestinationServerApi = os.Getenv("OCTOPUS_CLI_API_KEY")
	}

	defaultDestinationServerSpace := os.Getenv("OCTOTERRAWIZ_DESTINATION_SPACE_ID")
	if defaultDestinationServerSpace == "" {
		defaultDestinationServerSpace = "Spaces-1"
	}

	wiz.ShowWizardStep(steps.WelcomeStep{
		Wizard: *wiz,
		BaseStep: steps.BaseStep{State: state.State{
			BackendType:                   os.Getenv("OCTOTERRAWIZ_BACKEND_TYPE"),
			Server:                        defaultSourceServer,
			ServerExternal:                os.Getenv("OCTOTERRAWIZ_SOURCE_SERVER_EXTERNAL"),
			ApiKey:                        defaultSourceServerApi,
			Space:                         defaultSourceServerSpace,
			DestinationServer:             defaultDestinationServer,
			DestinationServerExternal:     "",
			DestinationApiKey:             defaultDestinationServerApi,
			DestinationSpace:              defaultDestinationServerSpace,
			AwsAccessKey:                  os.Getenv("AWS_ACCESS_KEY_ID"),
			AwsSecretKey:                  os.Getenv("AWS_SECRET_ACCESS_KEY"),
			AwsS3Bucket:                   os.Getenv("AWS_DEFAULT_BUCKET"),
			AwsS3BucketRegion:             os.Getenv("AWS_DEFAULT_REGION"),
			PromptForDelete:               strings.ToLower(os.Getenv("OCTOTERRAWIZ_PROMPT_FOR_DELETE")) == "true",
			UseContainerImages:            strings.ToLower(os.Getenv("OCTOTERRAWIZ_USE_CONTAINER_IMAGES")) == "true",
			AzureResourceGroupName:        os.Getenv("OCTOTERRAWIZ_AZURE_RESOURCE_GROUP"),
			AzureStorageAccountName:       os.Getenv("OCTOTERRAWIZ_AZURE_STORAGE_ACCOUNT"),
			AzureContainerName:            os.Getenv("OCTOTERRAWIZ_AZURE_CONTAINER"),
			AzureSubscriptionId:           os.Getenv("AZURE_SUBSCRIPTION_ID"),
			AzureTenantId:                 os.Getenv("AZURE_TENANT_ID"),
			AzureApplicationId:            os.Getenv("AZURE_CLIENT_ID"),
			AzurePassword:                 os.Getenv("AZURE_CLIENT_SECRET"),
			ExcludeAllLibraryVariableSets: strings.ToLower(os.Getenv("OCTOTERRAWIZ_EXCLUDE_ALL_LIBRARY_VARIABLE_SETS")) == "true",
			EnableVariableSpreading:       false,
			DatabaseServer:                os.Getenv("OCTOTERRAWIZ_DATABASE_SERVER"),
			DatabaseUser:                  os.Getenv("OCTOTERRAWIZ_DATABASE_USER"),
			DatabasePass:                  os.Getenv("OCTOTERRAWIZ_DATABASE_PASS"),
			DatabasePort:                  os.Getenv("OCTOTERRAWIZ_DATABASE_PORT"),
			DatabaseName:                  os.Getenv("OCTOTERRAWIZ_DATABASE_NAME"),
			DatabaseMasterKey:             os.Getenv("OCTOTERRAWIZ_DATABASE_MASTERKEY"),
			EnableProjectRenaming:         strings.ToLower(os.Getenv("OCTOTERRAWIZ_ENABLE_PROJECT_RENAMING")) == "true",
		}},
	})
	wiz.Window.ShowAndRun()
}
