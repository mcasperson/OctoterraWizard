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
	"github.com/mcasperson/OctoterraWizard/internal/strutil"
	"github.com/mcasperson/OctoterraWizard/internal/validators"
	"github.com/mcasperson/OctoterraWizard/internal/wizard"
	"net/url"
	"strings"
)

type AwsTerraformStateStep struct {
	BaseStep
	Wizard    wizard.Wizard
	accessKey *widget.Entry
	secretKey *widget.Entry
	s3Bucket  *widget.Entry
	s3Region  *widget.Entry
	result    *widget.Label
	infinite  *widget.ProgressBarInfinite
	logs      *widget.Entry
	previous  *widget.Button
	next      *widget.Button
}

func (s AwsTerraformStateStep) GetContainer(parent fyne.Window) *fyne.Container {

	bottom, previous, next := s.BuildNavigation(func() {
		s.Wizard.ShowWizardStep(BackendSelectionStep{
			Wizard:   s.Wizard,
			BaseStep: BaseStep{State: s.getState()}})
	}, func() {
		s.result.SetText("🔵 Validating AWS credentials and S3 bucket.")
		s.infinite.Show()
		s.accessKey.Disable()
		s.secretKey.Disable()
		s.s3Bucket.Disable()
		s.s3Region.Disable()
		s.logs.Hide()
		s.logs.SetText("")
		s.next.Disable()
		s.previous.Disable()

		defer s.infinite.Hide()
		defer s.accessKey.Enable()
		defer s.secretKey.Enable()
		defer s.s3Bucket.Enable()
		defer s.s3Region.Enable()
		defer s.next.Enable()
		defer s.next.Enable()

		validationFailed := false
		if err := validators.ValidateAWS(s.getState()); err != nil {
			if err := logutil.WriteTextToFile("aws_terraform_state_error.txt", err.Error()); err != nil {
				fmt.Println("Failed to write error to file")
			}

			s.result.SetText("🔴 Unable to validate the credentials. Please check the Access Key, Secret Key, S3 Bucket Name, and S3 Bucket Region.")
			s.logs.SetText(err.Error())
			s.logs.Show()
			validationFailed = true
		}

		if err := validators.TestS3Bucket(s.getState()); err != nil {
			if err := logutil.WriteTextToFile("aws_terraform_state_error.txt", err.Error()); err != nil {
				fmt.Println("Failed to write error to file")
			}

			s.result.SetText("🔴 Unable to connect to the S3 bucket. Please check that the bucket exists and that the supplied credentials can access it.")
			s.logs.SetText(err.Error())
			s.logs.Show()
			validationFailed = true
		}

		nexCallback := func(proceed bool) {
			if proceed {
				s.Wizard.ShowWizardStep(AdvancedOptionsStep{
					Wizard:   s.Wizard,
					BaseStep: BaseStep{State: s.getState()}})
			}
		}

		if validationFailed {
			dialog.NewConfirm("AWS Validation failed", "Validation of the AWS details failed. Do you wish to continue anyway?", nexCallback, s.Wizard.Window).Show()
		} else {
			nexCallback(true)
		}

	})
	s.next = next
	s.previous = previous
	next.Disable()

	heading := widget.NewLabel("AWS S3 Terraform State")
	heading.TextStyle = fyne.TextStyle{Bold: true}

	label1 := widget.NewLabel(strutil.TrimMultilineWhitespace(`
		Terraform manages its state in an S3 bucket in AWS. Please provide the details of the S3 bucket that will be used to store the Terraform state.
	`))

	linkUrl, _ := url.Parse("https://developer.hashicorp.com/terraform/language/settings/backends/s3")
	link := widget.NewHyperlink("Learn more about the AWS S3 Terraform backend.", linkUrl)

	s.result = widget.NewLabel("")

	s.infinite = widget.NewProgressBarInfinite()
	s.infinite.Hide()
	s.infinite.Start()

	s.logs = widget.NewEntry()
	s.logs.SetMinRowsVisible(20)
	s.logs.MultiLine = true
	s.logs.Disable()
	s.logs.Hide()

	accessKeyLabel := widget.NewLabel("AWS Access Key")
	s.accessKey = widget.NewEntry()
	s.accessKey.SetPlaceHolder("")
	s.accessKey.SetText(s.State.AwsAccessKey)

	secretKeyLabel := widget.NewLabel("AWS Secret Key")
	s.secretKey = widget.NewPasswordEntry()
	s.secretKey.SetPlaceHolder("")
	s.secretKey.SetText(s.State.AwsSecretKey)

	s3BucketLabel := widget.NewLabel("AWS S3 Bucket Name")
	s.s3Bucket = widget.NewEntry()
	s.s3Bucket.SetPlaceHolder("my-bucket")
	s.s3Bucket.SetText(s.State.AwsS3Bucket)

	apiKeyLabel := widget.NewLabel("AWS S3 Bucket Region")
	s.s3Region = widget.NewEntry()
	s.s3Region.SetPlaceHolder("us-east-1")
	s.s3Region.SetText(s.State.AwsS3BucketRegion)

	validation := func(input string) {
		if s.accessKey != nil && s.accessKey.Text != "" && s.secretKey != nil && s.secretKey.Text != "" && s.s3Bucket != nil && s.s3Bucket.Text != "" && s.s3Region != nil && s.s3Region.Text != "" {
			next.Enable()
		} else {
			next.Disable()
		}
	}

	validation("")

	s.accessKey.OnChanged = validation
	s.secretKey.OnChanged = validation
	s.s3Bucket.OnChanged = validation
	s.s3Region.OnChanged = validation

	formLayout := container.New(layout.NewFormLayout(), accessKeyLabel, s.accessKey, secretKeyLabel, s.secretKey, s3BucketLabel, s.s3Bucket, apiKeyLabel, s.s3Region)

	middle := container.New(layout.NewVBoxLayout(), heading, label1, link, formLayout, s.infinite, s.result, s.logs)

	content := container.NewBorder(nil, bottom, nil, nil, middle)

	return content
}

func (s AwsTerraformStateStep) getState() state.State {
	return state.State{
		BackendType:               s.State.BackendType,
		Server:                    s.State.Server,
		ServerExternal:            s.State.ServerExternal,
		ApiKey:                    s.State.ApiKey,
		Space:                     s.State.Space,
		DestinationServer:         s.State.DestinationServer,
		DestinationServerExternal: s.State.DestinationServerExternal,
		DestinationApiKey:         s.State.DestinationApiKey,
		DestinationSpace:          s.State.DestinationSpace,
		AwsAccessKey:              strings.TrimSpace(s.accessKey.Text),
		AwsSecretKey:              strings.TrimSpace(s.secretKey.Text),
		AwsS3Bucket:               strings.TrimSpace(s.s3Bucket.Text),
		AwsS3BucketRegion:         strings.TrimSpace(s.s3Region.Text),
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
