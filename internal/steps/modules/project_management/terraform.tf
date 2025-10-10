terraform {
  required_providers {
    octopusdeploy = { source = "OctopusDeploy/octopusdeploy", version = "1.3.10" }
  }
}

provider "octopusdeploy" {
  address  = var.octopus_server_external
  api_key  = var.octopus_apikey
  space_id = var.octopus_space_id
}

variable "octopus_server_external" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The URL of the Octopus server when accessed from the wizard. Will usually only different that octopus_server from tests"
}

variable "octopus_server" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The URL of the Octopus server e.g. https://myinstance.octopus.app."
}
variable "octopus_apikey" {
  type        = string
  nullable    = false
  sensitive   = true
  description = "The API key used to access the Octopus server. See https://octopus.com/docs/octopus-rest-api/how-to-create-an-api-key for details on creating an API key."
}
variable "octopus_space_id" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The space ID to populate"
}
variable "octopus_project_id" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The project to add the runbook to"
}
variable "octopus_destination_server" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The URL of the Octopus server e.g. https://myinstance.octopus.app."
}
variable "octopus_destination_apikey" {
  type        = string
  nullable    = false
  sensitive   = true
  description = "The API key used to access the Octopus server. See https://octopus.com/docs/octopus-rest-api/how-to-create-an-api-key for details on creating an API key."
}
variable "octopus_destination_space_id" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The space ID to populate"
}
variable "octopus_serialize_actiontemplateid" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The ID of the step template used to serialize a space"
}
variable "octopus_deploys3_actiontemplateid" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The ID of the step template used to deploy a space"
}
variable "octopus_deployazure_actiontemplateid" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The ID of the step template used to deploy a space"
}
variable "terraform_state_bucket" {
  type        = string
  nullable    = true
  sensitive   = false
  description = "The S3 bucket used to save Terraform state"
}
variable "terraform_state_bucket_region" {
  type        = string
  nullable    = true
  sensitive   = false
  description = "The S3 bucket region used to save Terraform state"
}
variable "octopus_project_name" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The project name to apply"
}
variable "terraform_state_azure_resource_group" {
  type        = string
  nullable    = true
  sensitive   = false
  description = "The Azure resource group holding the storage account used by the terraform state"
}

variable "terraform_state_azure_storage_account" {
  type        = string
  nullable    = true
  sensitive   = false
  description = "The Azure storage account used by the terraform state"
}

variable "terraform_state_azure_storage_container" {
  type        = string
  nullable    = true
  sensitive   = false
  description = "The Azure storage account container used by the terraform state"
}

variable "terraform_backend" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "The terraform backend to use"
  default     = "AWS S3"
}

variable "use_container_images" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "Whether to use container images or not"
  default     = "True"
}

variable "default_secret_variables" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "Whether to set sensitive variables to Octostache template or not"
  default     = "False"
}

variable "customise_destination_project_name" {
  type        = string
  nullable    = false
  sensitive   = false
  description = "Whether to allow the destination project name to be customised or not"
  default     = "False"
}

data "octopusdeploy_feeds" "docker_feed" {
  feed_type    = "Docker"
  ids          = null
  partial_name = "Octoterra Docker Feed"
  skip         = 0
  take         = 1
}

data "octopusdeploy_feeds" "built_in_feed" {
  feed_type    = "BuiltIn"
  ids          = null
  partial_name = "Octopus Server (built-in)"
  skip         = 0
  take         = 1
}

data "octopusdeploy_worker_pools" "ubuntu_worker_pool" {
  name = "Hosted Ubuntu"
  ids  = null
  skip = 0
  take = 1
}

resource "octopusdeploy_variable" "destination_project_name" {
  count = lower(var.customise_destination_project_name) == "true" ? 1 : 0
  owner_id     = var.octopus_project_id
  value        = "#{Octopus.Project.Name}"
  name         = "OctoterraWiz.Destination.ProjectName"
  type         = "String"
  description  = "Defines the name of the destination project."
  is_sensitive = false

  prompt {
    description = "The name of the destination project."
    label       = "Destination Project Name"
    is_required = true

    display_settings {
      control_type = "SingleLineText"
    }
  }
}


resource "octopusdeploy_runbook" "runbook" {
  project_id         = var.octopus_project_id
  name               = "__ 1. Serialize Project"
  description        = "Serialize the project to a Terraform module"
  multi_tenancy_mode = "Untenanted"
  connectivity_policy {
    allow_deployments_to_no_targets = false
    exclude_unhealthy_targets       = false
    skip_machine_behavior           = "SkipUnavailableMachines"
  }
  retention_policy {
    quantity_to_keep = 10
  }
  environment_scope           = "All"
  environments                = []
  default_guided_failure_mode = "EnvironmentDefault"
  force_package_download      = false
}

resource "octopusdeploy_runbook_process" "runbook" {
  runbook_id = octopusdeploy_runbook.runbook.id

  step {
    condition           = "Success"
    name                = "Octopus - Serialize Project to Terraform"
    package_requirement = "LetOctopusDecide"
    start_trigger       = "StartAfterPrevious"

    action {
      action_type                        = "Octopus.Script"
      name                               = "Octopus - Serialize Project to Terraform"
      condition                          = "Success"
      run_on_server                      = true
      is_disabled                        = false
      can_be_used_for_project_versioning = false
      is_required                        = false
      # Use the ubuntu worker pool if it is present, or use the default otherwise
      worker_pool_id = length(data.octopusdeploy_worker_pools.ubuntu_worker_pool.worker_pools) == 0 ? "" : data.octopusdeploy_worker_pools.ubuntu_worker_pool.worker_pools[0].id
      properties                         = {
        "SerializeProject.ThisInstance.Server.Url" = "#{Octopus.Source.Server}"
        "Octopus.Action.Template.Id" = var.octopus_serialize_actiontemplateid
        "SerializeProject.ThisInstance.Terraform.Backend" = var.terraform_backend == "AWS S3" ? "s3" : "azurerm"
        "Octopus.Action.Template.Version" = "1"
        "SerializeProject.Exported.Project.Name" = "#{Octopus.Project.Name}"
        "Octopus.Action.Script.Syntax" = "Python"
        "Octopus.Action.RunOnServer" = "true"
        "Octopus.Action.Script.ScriptSource" = "Inline"
        "Octopus.Action.Script.ScriptBody": "import argparse\nimport os\nimport stat\nimport re\nimport socket\nimport subprocess\nimport sys\nfrom datetime import datetime\nfrom urllib.parse import urlparse\nimport urllib.request\nfrom itertools import chain\nimport platform\nfrom urllib.request import urlretrieve\nimport zipfile\nimport json\nimport tarfile\nimport random, time\n\n# If this script is not being run as part of an Octopus step, return variables from environment variables.\n# Periods are replaced with underscores, and the variable name is converted to uppercase\nif \"get_octopusvariable\" not in globals():\n    def get_octopusvariable(variable):\n        return os.environ[re.sub('\\\\.', '_', variable.upper())]\n\n# If this script is not being run as part of an Octopus step, print directly to std out.\nif \"printverbose\" not in globals():\n    def printverbose(msg):\n        print(msg)\n\n\ndef printverbose_noansi(output):\n    \"\"\"\n    Strip ANSI color codes and print the output as verbose\n    :param output: The output to print\n    \"\"\"\n    if not output:\n        return\n\n    # https://stackoverflow.com/questions/14693701/how-can-i-remove-the-ansi-escape-sequences-from-a-string-in-python\n    output_no_ansi = re.sub(r'\\x1B(?:[@-Z\\\\-_]|\\[[0-?]*[ -/]*[@-~])', '', output)\n    printverbose(output_no_ansi)\n\n\ndef get_octopusvariable_quiet(variable):\n    \"\"\"\n    Gets an octopus variable, or an empty string if it does not exist.\n    :param variable: The variable name\n    :return: The variable value, or an empty string if the variable does not exist\n    \"\"\"\n    try:\n        return get_octopusvariable(variable)\n    except:\n        return ''\n\n\ndef retry_with_backoff(fn, retries=5, backoff_in_seconds=1):\n    x = 0\n    while True:\n        try:\n            return fn()\n        except Exception as e:\n\n            print(e)\n\n            if x == retries:\n                raise\n\n            sleep = (backoff_in_seconds * 2 ** x +\n                     random.uniform(0, 1))\n            time.sleep(sleep)\n            x += 1\n\n\ndef execute(args, cwd=None, env=None, print_args=None, print_output=printverbose_noansi):\n    \"\"\"\n        The execute method provides the ability to execute external processes while capturing and returning the\n        output to std err and std out and exit code.\n    \"\"\"\n    process = subprocess.Popen(args,\n                               stdout=subprocess.PIPE,\n                               stderr=subprocess.PIPE,\n                               text=True,\n                               cwd=cwd,\n                               env=env)\n    stdout, stderr = process.communicate()\n    retcode = process.returncode\n\n    if print_args is not None:\n        print_output(' '.join(args))\n\n    if print_output is not None:\n        print_output(stdout)\n        print_output(stderr)\n\n    return stdout, stderr, retcode\n\n\ndef is_windows():\n    return platform.system() == 'Windows'\n\n\ndef init_argparse():\n    parser = argparse.ArgumentParser(\n        usage='%(prog)s [OPTION] [FILE]...',\n        description='Serialize an Octopus project to a Terraform module'\n    )\n    parser.add_argument('--ignore-all-changes',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IgnoreAllChanges') or get_octopusvariable_quiet(\n                            'Exported.Project.IgnoreAllChanges') or 'false',\n                        help='Set to true to set the \"lifecycle.ignore_changes\" ' +\n                             'setting on each exported resource to \"all\"')\n    parser.add_argument('--ignore-variable-changes',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IgnoreVariableChanges') or get_octopusvariable_quiet(\n                            'Exported.Project.IgnoreVariableChanges') or 'false',\n                        help='Set to true to set the \"lifecycle.ignore_changes\" ' +\n                             'setting on each exported octopus variable to \"all\"')\n    parser.add_argument('--terraform-backend',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.ThisInstance.Terraform.Backend') or get_octopusvariable_quiet(\n                            'ThisInstance.Terraform.Backend') or 'pg',\n                        help='Set this to the name of the Terraform backend to be included in the generated module.')\n    parser.add_argument('--server-url',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.ThisInstance.Server.Url') or get_octopusvariable_quiet(\n                            'ThisInstance.Server.Url'),\n                        help='Sets the server URL that holds the project to be serialized.')\n    parser.add_argument('--api-key',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.ThisInstance.Api.Key') or get_octopusvariable_quiet(\n                            'ThisInstance.Api.Key'),\n                        help='Sets the Octopus API key.')\n    parser.add_argument('--space-id',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Space.Id') or get_octopusvariable_quiet(\n                            'Exported.Space.Id') or get_octopusvariable_quiet('Octopus.Space.Id'),\n                        help='Set this to the space ID containing the project to be serialized.')\n    parser.add_argument('--project-name',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.Name') or get_octopusvariable_quiet(\n                            'Exported.Project.Name') or get_octopusvariable_quiet(\n                            'Octopus.Project.Name'),\n                        help='Set this to the name of the project to be serialized.')\n    parser.add_argument('--upload-space-id',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Octopus.UploadSpace.Id') or get_octopusvariable_quiet(\n                            'Octopus.UploadSpace.Id') or get_octopusvariable_quiet('Octopus.Space.Id'),\n                        help='Set this to the space ID of the Octopus space where ' +\n                             'the resulting package will be uploaded to.')\n    parser.add_argument('--ignore-cac-managed-values',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IgnoreCacValues') or get_octopusvariable_quiet(\n                            'Exported.Project.IgnoreCacValues') or 'false',\n                        help='Set this to true to exclude cac managed values like non-secret variables, ' +\n                             'deployment processes, and project versioning into the Terraform module. ' +\n                             'Set to false to have these values embedded into the module.')\n    parser.add_argument('--exclude-cac-project-settings',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.ExcludeCacProjectValues') or get_octopusvariable_quiet(\n                            'Exported.Project.ExcludeCacProjectValues') or 'false',\n                        help='Set this to true to exclude CaC settings like git connections from the exported module.')\n    parser.add_argument('--ignored-library-variable-sets',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IgnoredLibraryVariableSet') or get_octopusvariable_quiet(\n                            'Exported.Project.IgnoredLibraryVariableSet'),\n                        help='A comma separated list of library variable sets to ignore.')\n    parser.add_argument('--ignored-accounts',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IgnoredAccounts') or get_octopusvariable_quiet(\n                            'Exported.Project.IgnoredAccounts'),\n                        help='A comma separated list of accounts to ignore.')\n    parser.add_argument('--ignored-tenants',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IgnoredTenants') or get_octopusvariable_quiet(\n                            'Exported.Project.IgnoredTenants'),\n                        help='A comma separated list of tenants to ignore.')\n    parser.add_argument('--ignored-channels',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IgnoredChannels') or get_octopusvariable_quiet(\n                            'Exported.Project.IgnoredChannels'),\n                        help='A comma separated list of channels to ignore.')\n    parser.add_argument('--include-step-templates',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.IncludeStepTemplates') or get_octopusvariable_quiet(\n                            'Exported.Project.IncludeStepTemplates') or 'false',\n                        help='Set this to true to include step templates in the exported module. ' +\n                             'This disables the default behaviour of detaching step templates.')\n    parser.add_argument('--lookup-project-link-tenants',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.LookupProjectLinkTenants') or get_octopusvariable_quiet(\n                            'Exported.Project.LookupProjectLinkTenants') or 'false',\n                        help='Set this option to link tenants and create tenant project variables.')\n    parser.add_argument('--default-secret-variables',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.DefaultSecrets') or get_octopusvariable_quiet(\n                            'Exported.Project.DefaultSecrets') or 'false',\n                        help='Set to true to set sensitive variables to the octostache template that represents the variable')\n    parser.add_argument('--octopus-managed-terraform-vars',\n                        action='store',\n                        default=get_octopusvariable_quiet(\n                            'SerializeProject.Exported.Project.OctopusManagedTerraformVars') or get_octopusvariable_quiet(\n                            'Exported.Project.OctopusManagedTerraformVars'),\n                        help='The name of an Octopus variable to use as the terraform.tfvars file.')\n\n    return parser.parse_known_args()\n\n\ndef get_latest_github_release(owner, repo, filename):\n    url = f\"https://api.github.com/repos/{owner}/{repo}/releases/latest\"\n    releases = urllib.request.urlopen(url).read()\n    contents = json.loads(releases)\n\n    download = [asset for asset in contents.get('assets') if asset.get('name') == filename]\n\n    if len(download) != 0:\n        return download[0].get('browser_download_url')\n\n    return None\n\n\ndef ensure_octo_cli_exists():\n    if is_windows():\n        print(\"Checking for the Octopus CLI\")\n        try:\n            stdout, _, exit_code = execute(['octo.exe', 'help'])\n            printverbose(stdout)\n            if not exit_code == 0:\n                raise \"Octo CLI not found\"\n            return \"\"\n        except:\n            print(\"Downloading the Octopus CLI\")\n            urlretrieve('https://download.octopusdeploy.com/octopus-tools/9.0.0/OctopusTools.9.0.0.win-x64.zip',\n                        'OctopusTools.zip')\n            with zipfile.ZipFile('OctopusTools.zip', 'r') as zip_ref:\n                zip_ref.extractall(os.getcwd())\n            return os.getcwd()\n    else:\n        print(\"Checking for the Octopus CLI for Linux\")\n        try:\n            stdout, _, exit_code = execute(['octo', 'help'])\n            printverbose(stdout)\n            if not exit_code == 0:\n                raise \"Octo CLI not found\"\n            return \"\"\n        except:\n            print(\"Downloading the Octopus CLI for Linux\")\n            urlretrieve('https://download.octopusdeploy.com/octopus-tools/9.0.0/OctopusTools.9.0.0.linux-x64.tar.gz',\n                        'OctopusTools.tar.gz')\n            with tarfile.open('OctopusTools.tar.gz') as file:\n                file.extractall(os.getcwd())\n                os.chmod(os.path.join(os.getcwd(), 'octo'), stat.S_IRWXO | stat.S_IRWXU | stat.S_IRWXG)\n            return os.getcwd()\n\n\ndef ensure_octoterra_exists():\n    if is_windows():\n        print(\"Checking for the Octoterra tool for Windows\")\n        try:\n            stdout, _, exit_code = execute(['octoterra.exe', '-version'])\n            printverbose(stdout)\n            if not exit_code == 0:\n                raise \"Octoterra not found\"\n            return \"\"\n        except:\n            print(\"Downloading Octoterra CLI for Windows\")\n            retry_with_backoff(lambda: urlretrieve(\n                \"https://github.com/OctopusSolutionsEngineering/OctopusTerraformExport/releases/latest/download/octoterra_windows_amd64.exe\",\n                'octoterra.exe'), 10, 30)\n            return os.getcwd()\n    else:\n        print(\"Checking for the Octoterra tool for Linux\")\n        try:\n            stdout, _, exit_code = execute(['octoterra', '-version'])\n            printverbose(stdout)\n            if not exit_code == 0:\n                raise \"Octoterra not found\"\n            return \"\"\n        except:\n            print(\"Downloading Octoterra CLI for Linux\")\n            retry_with_backoff(lambda: urlretrieve(\n                \"https://github.com/OctopusSolutionsEngineering/OctopusTerraformExport/releases/latest/download/octoterra_linux_amd64\",\n                'octoterra'), 10, 30)\n            os.chmod(os.path.join(os.getcwd(), 'octoterra'), stat.S_IRWXO | stat.S_IRWXU | stat.S_IRWXG)\n            return os.getcwd()\n\n\noctocli_path = ensure_octo_cli_exists()\noctoterra_path = ensure_octoterra_exists()\nparser, _ = init_argparse()\n\n# Variable precondition checks\nif len(parser.server_url) == 0:\n    print(\"--server-url, ThisInstance.Server.Url, or SerializeProject.ThisInstance.Server.Url must be defined\")\n    sys.exit(1)\n\nif len(parser.api_key) == 0:\n    print(\"--api-key, ThisInstance.Api.Key, or ThisInstance.Api.Key must be defined\")\n    sys.exit(1)\n\nprint(\"Octopus URL: \" + parser.server_url)\nprint(\"Octopus Space ID: \" + parser.space_id)\n\n# Build the arguments to ignore library variable sets\nignores_library_variable_sets = parser.ignored_library_variable_sets.split(',')\nignores_library_variable_sets_args = [['-excludeLibraryVariableSet', x] for x in ignores_library_variable_sets]\n\n# Build the arguments to ignore accounts\nignored_accounts = parser.ignored_accounts.split(',')\nignored_accounts = [['-excludeAccounts', x] for x in ignored_accounts]\n\n# Build the arguments to ignore tenants\nignored_tenants = parser.ignored_tenants.split(',')\nignored_tenants_args = [['-excludeTenants', x] for x in ignored_tenants]\n\n# Build the arguments to ignore channels\nignored_channels = parser.ignored_channels.split(',')\nignored_channels_args = [['-excludeChannels', x] for x in ignored_channels]\n\nos.mkdir(os.getcwd() + '/export')\n\nexport_args = [os.path.join(octoterra_path, 'octoterra'),\n               # the url of the instance\n               '-url', parser.server_url,\n               # the api key used to access the instance\n               '-apiKey', parser.api_key,\n               # add a postgres backend to the generated modules\n               '-terraformBackend', parser.terraform_backend,\n               # dump the generated HCL to the console\n               '-console',\n               # dump the project from the current space\n               '-space', parser.space_id,\n               # the name of the project to serialize\n               '-projectName', parser.project_name,\n               # ignoreProjectChanges can be set to ignore all changes to the project, variables, runbooks etc\n               '-ignoreProjectChanges=' + parser.ignore_all_changes,\n               # use data sources to lookup external dependencies (like environments, accounts etc) rather\n               # than serialize those external resources\n               '-lookupProjectDependencies',\n               # for any secret variables, add a default value set to the octostache value of the variable\n               # e.g. a secret variable called \"database\" has a default value of \"#{database}\"\n               '-defaultSecretVariableValues=' + parser.default_secret_variables,\n               # Any value that can't be replaced with an Octostache template, add a dummy value\n               '-dummySecretVariableValues',\n               # detach any step templates, allowing the exported project to be used in a new space\n               '-detachProjectTemplates=' + str(not parser.include_step_templates),\n               # allow the downstream project to move between project groups\n               '-ignoreProjectGroupChanges',\n               # allow the downstream project to change names\n               '-ignoreProjectNameChanges',\n               # CaC enabled projects will not export the deployment process, non-secret variables, and other\n               # CaC managed project settings if ignoreCacManagedValues is true. It is usually desirable to\n               # set this value to true, but it is false here because CaC projects created by Terraform today\n               # save some variables in the database rather than writing them to the Git repo.\n               '-ignoreCacManagedValues=' + parser.ignore_cac_managed_values,\n               # Excluding CaC values means the resulting module does not include things like git credentials.\n               # Setting excludeCaCProjectSettings to true and ignoreCacManagedValues to false essentially\n               # converts a CaC project back to a database project.\n               '-excludeCaCProjectSettings=' + parser.exclude_cac_project_settings,\n               # This value is always true. Either this is an unmanaged project, in which case we are never\n               # reapplying it; or it is a variable configured project, in which case we need to ignore\n               # variable changes, or it is a shared CaC project, in which case we don't use Terraform to\n               # manage variables.\n               '-ignoreProjectVariableChanges=' + parser.ignore_variable_changes,\n               # To have secret variables available when applying a downstream project, they must be scoped\n               # to the Sync environment. But we do not need this scoping in the downstream project, so the\n               # Sync environment is removed from any variable scopes when serializing it to Terraform.\n               '-excludeVariableEnvironmentScopes', 'Sync',\n               # Exclude any variables starting with \"Private.\"\n               '-excludeProjectVariableRegex', 'Private\\\\..*',\n               # Capture the octopus endpoint, space ID, and space name as output vars. This is useful when\n               # querying th Terraform state file to know which space and instance the resources were\n               # created in. The scripts used to update downstream projects in bulk work by querying the\n               # Terraform state, finding all the downstream projects, and using the space name to only process\n               # resources that match the current tenant (because space names and tenant names are the same).\n               # The output variables added by this option are octopus_server, octopus_space_id, and\n               # octopus_space_name.\n               '-includeOctopusOutputVars',\n               # Where steps do not explicitly define a worker pool and reference the default one, this\n               # option explicitly exports the default worker pool by name. This means if two spaces have\n               # different default pools, the exported project still uses the pool that the original project\n               # used.\n               '-lookUpDefaultWorkerPools',\n               # Link any tenants that were originally link to the project and create project tenant variables\n               '-lookupProjectLinkTenants=' + parser.lookup_project_link_tenants,\n               # Add support for experimental step templates\n               '-experimentalEnableStepTemplates=' + parser.include_step_templates,\n               # Ignore invalid channels\n               '-excludeInvalidChannels',\n               # The directory where the exported files will be saved\n               '-dest', os.getcwd() + '/export',\n               # Define the name of an Octopus variable to populate the terraform.tfvars file\n               '-octopusManagedTerraformVars=' + parser.octopus_managed_terraform_vars,\n               # This is a management runbook that we do not wish to export\n               '-excludeRunbookRegex', '__ .*'] + list(chain(*ignores_library_variable_sets_args)) + list(\n    chain(*ignored_accounts)) + list(chain(*ignored_tenants_args)) + list(chain(*ignored_channels_args))\n\nprint(\"Exporting Terraform module\")\n_, _, octoterra_exit = execute(export_args)\n\nif not octoterra_exit == 0:\n    print(\"Octoterra failed. Please check the logs for more information.\")\n    sys.exit(1)\n\ndate = datetime.now().strftime('%Y.%m.%d.%H%M%S')\n\nprint(\"Creating Terraform module package\")\nif is_windows():\n    execute([os.path.join(octocli_path, 'octo.exe'),\n             'pack',\n             '--format', 'zip',\n             '--id', re.sub('[^0-9a-zA-Z]', '_', parser.project_name),\n             '--version', date,\n             '--basePath', os.getcwd() + '\\\\export',\n             '--outFolder', os.getcwd()])\nelse:\n    _, _, _ = execute([os.path.join(octocli_path, 'octo'),\n                       'pack',\n                       '--format', 'zip',\n                       '--id', re.sub('[^0-9a-zA-Z]', '_', parser.project_name),\n                       '--version', date,\n                       '--basePath', os.getcwd() + '/export',\n                       '--outFolder', os.getcwd()])\n\nprint(\"Uploading Terraform module package\")\nif is_windows():\n    _, _, _ = execute([os.path.join(octocli_path, 'octo.exe'),\n                       'push',\n                       '--apiKey', parser.api_key,\n                       '--server', parser.server_url,\n                       '--space', parser.upload_space_id,\n                       '--package', os.getcwd() + \"\\\\\" +\n                       re.sub('[^0-9a-zA-Z]', '_', parser.project_name) + '.' + date + '.zip',\n                       '--replace-existing'])\nelse:\n    _, _, _ = execute([os.path.join(octocli_path, 'octo'),\n                       'push',\n                       '--apiKey', parser.api_key,\n                       '--server', parser.server_url,\n                       '--space', parser.upload_space_id,\n                       '--package', os.getcwd() + \"/\" +\n                       re.sub('[^0-9a-zA-Z]', '_', parser.project_name) + '.' + date + '.zip',\n                       '--replace-existing'])\n\nprint(\"##octopus[stdout-default]\")\n\nprint(\"Done\")\n"
        "SerializeProject.Exported.Space.Id" = "#{Octopus.Space.Id}"
        "SerializeProject.Exported.Project.IgnoreVariableChanges" = "False"
        "SerializeProject.Exported.Project.IgnoreCacValues" = "False"
        "Exported.Project.IgnoredLibraryVariableSet" = "Octoterra,SpaceSensitiveVars"
        "SerializeProject.Exported.Project.IgnoreAllChanges" = "False"
        "SerializeProject.Project.Space.DefaultSecrets" = var.default_secret_variables
        "SerializeProject.ThisInstance.Api.Key" = "#{Octopus.Source.ApiKey}"
        "SerializeProject.Exported.Project.IncludeStepTemplates" = "True"
        "SerializeProject.Exported.Project.IgnoredAccounts" = "Octoterra AWS Account,Octoterra Azure Account"
        "SerializeProject.Exported.Project.IgnoredTenants" = ""
        "SerializeProject.Exported.Project.ExcludeCacProjectValues" = "True"
        "SerializeProject.Exported.Project.LookupProjectLinkTenants" = "True"
        "Octopus.Action.AutoRetry.MaximumCount"                 = "3"
        "SerializeProject.Exported.Project.OctopusManagedTerraformVars" = "OctoterraWiz.Terraform.Vars"
      }
      environments                       = []
      excluded_environments              = []
      channels                           = []
      tenant_tags                        = []
      features                           = []
    }

    properties   = {}
    target_roles = []
  }
}

resource "octopusdeploy_runbook" "deploy_project" {
  project_id         = var.octopus_project_id
  name               = "__ 2. Deploy Project"
  description        = "Deploy the serialized Terraform module to a space"
  multi_tenancy_mode = "Untenanted"
  connectivity_policy {
    allow_deployments_to_no_targets = false
    exclude_unhealthy_targets       = false
    skip_machine_behavior           = "SkipUnavailableMachines"
  }
  retention_policy {
    quantity_to_keep = 10
  }
  environment_scope           = "All"
  environments                = []
  default_guided_failure_mode = "EnvironmentDefault"
  force_package_download      = false
}

resource "octopusdeploy_runbook_process" "deploy_project_aws" {
  count                             = var.terraform_backend == "AWS S3" ? 1 : 0
  runbook_id = octopusdeploy_runbook.deploy_project.id

  step {
    condition           = "Success"
    name                = "Octopus - Populate Octoterra Space (S3 Backend)"
    package_requirement = "LetOctopusDecide"
    start_trigger       = "StartAfterPrevious"

    action {
      action_type                        = "Octopus.TerraformApply"
      name                               = "Octopus - Populate Octoterra Space (S3 Backend)"
      condition                          = "Success"
      run_on_server                      = true
      is_disabled                        = false
      can_be_used_for_project_versioning = true
      is_required                        = false
      # Use the ubuntu worker pool if it is present, or use the default otherwise
      worker_pool_id = length(data.octopusdeploy_worker_pools.ubuntu_worker_pool.worker_pools) == 0 ? "" : data.octopusdeploy_worker_pools.ubuntu_worker_pool.worker_pools[0].id
      properties                         = {
        "OctoterraApply.AWS.S3.BucketName" = var.terraform_state_bucket
        "OctoterraApply.AWS.S3.BucketRegion" =  var.terraform_state_bucket_region
        "OctoterraApply.AWS.Account" = "Terraform.AWS.Account"
        "OctoterraApply.AWS.S3.BucketKey" = "Project_#{if OctoterraWiz.Destination.ProjectName}#{OctoterraWiz.Destination.ProjectName | Replace \"[^A-Za-z0-9]\" \"_\"}#{/if}#{unless OctoterraWiz.Destination.ProjectName}#{Octopus.Project.Name | Replace \"[^A-Za-z0-9]\" \"_\"}#{/unless}"
        "Octopus.Action.Terraform.Workspace" = "#{OctoterraApply.Terraform.Workspace.Name}"
        "Octopus.Action.AwsAccount.UseInstanceRole" = "False"
        "Octopus.Action.AwsAccount.Variable" = "#{OctoterraApply.AWS.Account}"
        "Octopus.Action.Aws.Region" = "#{OctoterraApply.AWS.S3.BucketRegion}"
        "Octopus.Action.Template.Id" = var.octopus_deploys3_actiontemplateid
        "Octopus.Action.Terraform.RunAutomaticFileSubstitution" = "False"
        "Octopus.Action.Terraform.AdditionalInitParams" = "-backend-config=\"bucket=#{OctoterraApply.AWS.S3.BucketName}\" -backend-config=\"region=#{OctoterraApply.AWS.S3.BucketRegion}\" -backend-config=\"key=#{OctoterraApply.AWS.S3.BucketKey}\" #{if OctoterraApply.Terraform.AdditionalInitParams}#{OctoterraApply.Terraform.AdditionalInitParams}#{/if}"
        "Octopus.Action.Terraform.TemplateDirectory" = "space_population"
        "Octopus.Action.Package.DownloadOnTentacle" = "False"
        "Octopus.Action.Terraform.AllowPluginDownloads" = "True"
        "OctoterraApply.Octopus.ServerUrl" = "#{Octopus.Destination.Server}"
        "Octopus.Action.RunOnServer" = "true"
        "Octopus.Action.Terraform.PlanJsonOutput" = "False"
        "Octopus.Action.Terraform.AzureAccount" = "False"
        "OctoterraApply.Octopus.ApiKey" = "#{Octopus.Destination.ApiKey}"
        "Octopus.Action.GoogleCloud.ImpersonateServiceAccount" = "False"
        "Octopus.Action.Terraform.GoogleCloudAccount" = "False"
        # If a project name is set, infer the terraform variable that defines the project name, and set it to the destination project name.
        "OctoterraApply.Terraform.AdditionalApplyParams" = "#{if OctoterraWiz.Destination.ProjectName}\"-var=project_#{Octopus.Project.Name | Replace \"[^A-Za-z0-9]\" \"_\" | ToLower}_name=#{OctoterraWiz.Destination.ProjectName}\"#{/if}"
        "Octopus.Action.Terraform.AdditionalActionParams" = "-var=octopus_server=#{OctoterraApply.Octopus.ServerUrl} -var=octopus_apikey=#{OctoterraApply.Octopus.ApiKey} -var=octopus_space_id=#{OctoterraApply.Octopus.SpaceID} #{if OctoterraApply.Terraform.AdditionalApplyParams}#{OctoterraApply.Terraform.AdditionalApplyParams}#{/if}"
        "Octopus.Action.Terraform.FileSubstitution" = "**/project_variable_sensitive*.tf\n**/terraform.tfvars"
        "Octopus.Action.Script.ScriptSource" = "Package"
        "Octopus.Action.Template.Version" = "1"
        "Octopus.Action.GoogleCloud.UseVMServiceAccount" = "True"
        "Octopus.Action.Terraform.ManagedAccount" = "AWS"
        "Octopus.Action.Aws.AssumeRole" = "False"
        "OctoterraApply.Terraform.Package.Id" = jsonencode({
          "PackageId" = "${replace(var.octopus_project_name, "/[^A-Za-z0-9]/", "_")}"
          "FeedId" = "${data.octopusdeploy_feeds.built_in_feed.feeds[0].id}"
        })
        "OctoterraApply.Terraform.Workspace.Name" = "#{OctoterraApply.Octopus.SpaceID}"
        "OctoterraApply.Octopus.SpaceID" = "#{Octopus.Destination.SpaceID}"
        "OctopusUseBundledTooling" = "False"
        "Octopus.Action.AutoRetry.MaximumCount" = "3"
      }

      container {
        feed_id = lower(var.use_container_images) == "true" ? data.octopusdeploy_feeds.docker_feed.feeds[0].id : ""
        image   = lower(var.use_container_images) == "true" ? "ghcr.io/octopusdeploylabs/terraform-workertools" : ""
      }

      environments          = []
      excluded_environments = []
      channels              = []
      tenant_tags           = []

      primary_package {
        package_id           = "${replace(var.octopus_project_name, "/[^A-Za-z0-9]/", "_")}"
        acquisition_location = "Server"
        feed_id              = "${data.octopusdeploy_feeds.built_in_feed.feeds[0].id}"
        properties           = { PackageParameterName = "OctoterraApply.Terraform.Package.Id", SelectionMode = "deferred" }
      }

      features = []
    }

    properties   = {}
    target_roles = []
  }

}

resource "octopusdeploy_runbook_process" "deploy_project_azure" {
  runbook_id = octopusdeploy_runbook.deploy_project.id
  count      = var.terraform_backend == "Azure Storage" ? 1 : 0

  step {
    condition           = "Success"
    name                = "Octopus - Populate Octoterra Space (Azure Backend)"
    package_requirement = "LetOctopusDecide"
    start_trigger       = "StartAfterPrevious"

    action {
      action_type                        = "Octopus.TerraformApply"
      name                               = "Octopus - Populate Octoterra Space (Azure Backend)"
      condition                          = "Success"
      run_on_server                      = true
      is_disabled                        = false
      can_be_used_for_project_versioning = true
      is_required                        = false
      worker_pool_variable               = ""
      # Use the ubuntu worker pool if it is present, or use the default otherwise
      worker_pool_id                     = length(data.octopusdeploy_worker_pools.ubuntu_worker_pool.worker_pools) == 0 ? "" : data.octopusdeploy_worker_pools.ubuntu_worker_pool.worker_pools[0].id
      properties = {
        "Octopus.Action.Template.Id"                    = var.octopus_deployazure_actiontemplateid
        "Octopus.Action.Template.Version"               = "1"
        "Octopus.Action.RunOnServer"                    = "true"
        "Octopus.Action.Terraform.AllowPluginDownloads" = "True"
        "Octopus.Action.Package.DownloadOnTentacle"     = "False"
        "OctoterraApply.Terraform.Package.Id" = jsonencode({
          "PackageId" = "${replace(var.octopus_project_name, "/[^A-Za-z0-9]/", "_")}"
          "FeedId" = "${data.octopusdeploy_feeds.built_in_feed.feeds[0].id}"
        })
        "Octopus.Action.Terraform.Workspace"                    = "#{OctoterraApply.Terraform.Workspace.Name}"
        "Octopus.Action.Terraform.FileSubstitution"             = "**/project_variable_sensitive*.tf\n**/terraform.tfvars"
        "Octopus.Action.Aws.AssumeRole"                         = "False"
        "Octopus.Action.Terraform.TemplateDirectory"            = "space_population"
        "Octopus.Action.GoogleCloud.ImpersonateServiceAccount"  = "False"
        "Octopus.Action.AzureAccount.Variable"                  = "#{OctoterraApply.Azure.Account}"
        "OctoterraApply.Octopus.ServerUrl"                      = "#{Octopus.Destination.Server}"
        "OctoterraApply.Octopus.ApiKey"                         = "#{Octopus.Destination.ApiKey}"
        "OctoterraApply.Octopus.SpaceID"                        = "#{Octopus.Destination.SpaceID}"
        "OctoterraApply.Terraform.Workspace.Name"               = "#{OctoterraApply.Octopus.SpaceID}"
        "OctoterraApply.Azure.Storage.ResourceGroup"            = var.terraform_state_azure_resource_group
        "OctoterraApply.Azure.Storage.AccountName"              = var.terraform_state_azure_storage_account
        "OctoterraApply.Azure.Storage.Container"                = var.terraform_state_azure_storage_container
        "OctoterraApply.Azure.Storage.Key"                      = "Project_#{if OctoterraWiz.Destination.ProjectName}#{OctoterraWiz.Destination.ProjectName | Replace \"[^A-Za-z0-9]\" \"_\"}#{/if}#{unless OctoterraWiz.Destination.ProjectName}#{Octopus.Project.Name | Replace \"[^A-Za-z0-9]\" \"_\"}#{/unless}"
        "OctoterraApply.Azure.Account"                          = "Terraform.Azure.Account"
        # If a project name is set, infer the terraform variable that defines the project name, and set it to the destination project name.
        "OctoterraApply.Terraform.AdditionalApplyParams"        = "#{if OctoterraWiz.Destination.ProjectName}\"-var=project_#{Octopus.Project.Name | Replace \"[^A-Za-z0-9]\" \"_\" | ToLower}_name=#{OctoterraWiz.Destination.ProjectName}\"#{/if}"
        "Octopus.Action.Terraform.AdditionalActionParams"       = "-var=octopus_server=#{OctoterraApply.Octopus.ServerUrl} -var=octopus_apikey=#{OctoterraApply.Octopus.ApiKey} -var=octopus_space_id=#{OctoterraApply.Octopus.SpaceID} #{if OctoterraApply.Terraform.AdditionalApplyParams}#{OctoterraApply.Terraform.AdditionalApplyParams}#{/if}"
        "Octopus.Action.Script.ScriptSource"                    = "Package"
        "Octopus.Action.Terraform.GoogleCloudAccount"           = "False"
        "Octopus.Action.Terraform.RunAutomaticFileSubstitution" = "False"
        "Octopus.Action.Terraform.AzureAccount"                 = "True"
        "Octopus.Action.AwsAccount.UseInstanceRole"             = "False"
        "Octopus.Action.GoogleCloud.UseVMServiceAccount"        = "True"
        "Octopus.Action.Terraform.PlanJsonOutput"               = "False"
        "Octopus.Action.Terraform.ManagedAccount"               = "None"
        "Octopus.Action.Terraform.AdditionalInitParams"         = "-backend-config=\"resource_group_name=#{OctoterraApply.Azure.Storage.ResourceGroup}\" -backend-config=\"storage_account_name=#{OctoterraApply.Azure.Storage.AccountName}\" -backend-config=\"container_name=#{OctoterraApply.Azure.Storage.Container}\" -backend-config=\"key=#{OctoterraApply.Azure.Storage.Key}\" #{if OctoterraApply.Terraform.AdditionalInitParams}#{OctoterraApply.Terraform.AdditionalInitParams}#{/if}"
        "Octopus.Action.AutoRetry.MaximumCount"                 = "3"
      }

      container {
        feed_id = lower(var.use_container_images) == "true" ? data.octopusdeploy_feeds.docker_feed.feeds[0].id : ""
        image   = lower(var.use_container_images) == "true" ? "ghcr.io/octopusdeploylabs/terraform-workertools" : ""
      }

      environments = []
      excluded_environments = []
      channels = []
      tenant_tags = []

      primary_package {
        package_id           = "${replace(var.octopus_project_name, "/[^A-Za-z0-9]/", "_")}"
        acquisition_location = "Server"
        feed_id              = "${data.octopusdeploy_feeds.built_in_feed.feeds[0].id}"
        properties           = { PackageParameterName = "OctoterraApply.Terraform.Package.Id", SelectionMode = "deferred" }
      }

      features = []
    }

    properties = {}
    target_roles = []
  }

}