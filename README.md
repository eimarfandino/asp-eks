# asp-eks

## 🤔 Why?

I found myself constantly juggling between multiple AWS accounts and EKS clusters, going through the same tedious process:
- Remembering the correct AWS profile names
- Running `asp` to switch profiles
- Logging in via SSO
- Updating kubeconfig manually
- Trying to remember which clusters belonged to which account

This tool streamlines this entire workflow into a single command, making it easy to switch between different AWS accounts and their corresponding EKS clusters.

## 🎯 What it does

Tool for moving to a different EKS cluster using AWS profiles. It automatically:
- Switches AWS profiles
- Handles SSO login
- Updates kubeconfig
- Lists available clusters

## Prerequisites

- AWS CLI installed
- AWS SSO configured
- AWS profiles set up in `~/.aws/config`
- kubectl installed

## AWS Profile Configuration

Ensure your AWS profiles are properly configured in `~/.aws/config`:

```ini
[profile dev]
sso_start_url = https://your-sso-url.awsapps.com/start
sso_region = eu-central-1
sso_account_id = 123456789012
sso_role_name = YourRole
region = eu-central-1

[profile prod]
sso_start_url = https://your-sso-url.awsapps.com/start
sso_region = eu-central-1
sso_account_id = 987654321098
sso_role_name = YourRole
region = eu-central-1
```

## Usage

```bash
asp-eks [command]
```

### Available Commands

- `completion`: Generate the autocompletion script for the specified shell
- `generate-profiles`: Generate AWS profiles for all SSO accounts and roles
- `help`: Help about any command
- `list`: List available AWS profiles
- `use`: Use a specific AWS profile and set kubeconfig for an EKS cluster

### Generate Profiles Command


The `generate-profiles` command automatically creates AWS profiles for all accounts and roles accessible through your SSO configuration. This is particularly useful when you have access to multiple AWS accounts through SSO and want to avoid manually creating profiles for each account/role combination.

**Important:**
- If you do not have a `~/.aws/config` file, you must provide the `--sso-start-url` flag to specify your AWS SSO start URL. The tool will create a minimal config file for you.
- If you already have a config file with SSO configuration, the flag is optional and will override the SSO start URL for that run.

**Features:**
- Automatically discovers all accounts and roles available through SSO
- Generates profiles with consistent naming: `<account-name>-<role-name>`
- Supports dry-run to preview profiles before creating them
- Configurable default region for all generated profiles

**Options:**
- `--dry-run`: Show what profiles would be generated without writing to config file
- `--region, -r`: Default AWS region for generated profiles (default "eu-central-1")
- `--sso-start-url`: Override or set the SSO start URL for generated profiles (required if no config file exists)

**Prerequisites:**
- You must be logged in to AWS SSO (run `aws sso login --profile DEFAULT-SSO` after first run)
- You must have at least one SSO profile configured in `~/.aws/config`, or provide `--sso-start-url` to create one

**Examples:**
```bash
# Preview profiles that would be generated (with a new SSO start URL)
asp-eks generate-profiles --dry-run --sso-start-url https://your-sso-url.awsapps.com/start

# Generate profiles (will require --sso-start-url if no config exists)
asp-eks generate-profiles --sso-start-url https://your-sso-url.awsapps.com/start

# Generate profiles with custom default region
asp-eks generate-profiles --region eu-central-1 --sso-start-url https://your-sso-url.awsapps.com/start

# If you already have a config file, you can omit the flag:
asp-eks generate-profiles
```

### Use Command Options

The `use` command supports the following flag:

- `--dev`: Use development Azure configuration instead of production configuration

When using the `--dev` flag, the tool will configure Azure authentication with development-specific tenant and client IDs for kubelogin integration. This is useful when working with development environments that require different Azure AD configurations.

**Examples:**
```bash
# Use production Azure configuration (default)
asp-eks use my-profile

# Use development Azure configuration
asp-eks use my-profile --dev
```

## 🔧 Installation

### Download Latest Release (Recommended)

The most reliable way to install asp-eks is to download the latest release from GitLab:

1. **Download the latest release:**
   - Visit the releases page: https://github.com/eimarfandino/asp-eks/releases
   - Download the appropriate binary for your operating system

2. **Install the binary:**
   ```bash
   # Make the binary executable
   chmod +x asp-eks
   
   # Move to your PATH
   sudo mv asp-eks /usr/local/bin/
   ```

### Quick One-Line Installation

For a quick installation, you can use this one-liner to download and run the installation script:

```bash
# Set your GitLab token first
export GITLAB_TOKEN=your_token_here

# Download and run the installation script
curl -fsSL --header "PRIVATE-TOKEN: $GITLAB_TOKEN" https://gitlab.com/api/v4/projects/69875845/repository/files/install.sh/raw | bash
```

The script will automatically:
- Detect your operating system (macOS or Linux)
- Detect your processor architecture (Intel x86_64 or Apple Silicon arm64)
- Download the appropriate binary from the latest GitLab release
- Install it to `/usr/local/bin/`

**Prerequisites for one-liner:**
- `curl` installed
- `unzip` installed
- `GITLAB_TOKEN` environment variable set with your GitLab personal access token

**To set up your GitLab token:**
1. Create a personal access token in GitLab with `read_api` and `read_repository` scopes
2. Export it as an environment variable: `export GITLAB_TOKEN=your_token_here`

3. **Verify installation:**
   ```bash
   asp-eks --help
   ```

### Manual Installation (Build from Source)

If you prefer to build from source:

```bash
# Clone the repository
git clone https://github.com/eimarfandino/asp-eks.git

# Navigate to the directory
cd asp-eks

# Build the binary
go build -o asp-eks

# Move the binary to your PATH
sudo mv asp-eks /usr/local/bin/
```

### Examples

List available AWS profiles:
```bash
asp-eks list
```

Switch to a specific profile and configure EKS:
```bash
asp-eks use <profile-name>
```

Switch to a specific profile with development Azure configuration:
```bash
asp-eks use <profile-name> --dev
```

Get help for a specific command:
```bash
asp-eks [command] --help
```

## Global Flags

- `-h, --help`: Display help information for asp-eks

## Azure Integration

The tool includes Azure Active Directory integration for enhanced authentication. When using the `use` command, it automatically configures kubelogin for Azure AD authentication with the appropriate tenant and client configurations.

**Azure Configuration:**
- **Production (default)**: Uses production Azure AD tenant and client IDs
- **Development (`--dev` flag)**: Uses development Azure AD tenant and client IDs

This creates an additional Azure context named `entraid-<cluster-name>` that you can switch to for Azure AD authentication:
```bash
kubectl config use-context entraid-<cluster-name>
```

## Note

The tool will automatically:
1. Switch AWS profile
2. Handle SSO login if needed
3. Update kubeconfig for EKS cluster access

For more detailed information about a specific command, use:
```bash
asp-eks [command] --help
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 👥 Authors

- Eimar Fandino - Initial work