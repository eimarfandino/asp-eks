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
- `search`: Search for AWS profiles by name (case-insensitive substring match)
- `use`: Use a specific AWS profile and set kubeconfig for an EKS cluster

### Search Command

```bash
asp-eks search <query>
```

Case-insensitive substring search across all configured AWS profile names.

```bash
asp-eks search np20       # matches cluster-np20-dev, np20-dev-cluster, etc.
asp-eks search prod       # matches any profile containing "prod"
```

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

### Use Command

```bash
asp-eks use <profile-name>
```

If credentials are expired, `asp-eks` will automatically run `aws sso login --profile <profile>` and retry — no need to login manually first.

**Examples:**
```bash
# Switch profile and update kubeconfig
asp-eks use my-profile
```

#### Shell wrapper (recommended)

Because a subprocess cannot modify the parent shell's environment directly, the recommended way to also set `AWS_PROFILE` in your current shell is via a shell function in your `.zshrc` or `.bashrc`:

```sh
# ~/.zshrc
aeks() {
  asp-eks "$@"
  [[ "$1" == "use" ]] && export AWS_PROFILE="$2"
}
```

> **Note:** The name `asp` is taken by the oh-my-zsh `aws` plugin, hence `aeks`.

This wrapper supports all commands:
```bash
aeks use my-profile   # switches profile, updates kubeconfig, exports AWS_PROFILE
aeks list             # lists available profiles
```

## 🔧 Installation

### Download Latest Release (Recommended)

1. Visit the releases page: https://github.com/eimarfandino/asp-eks/releases
2. Download the appropriate binary for your operating system

```bash
# Make the binary executable
chmod +x asp-eks

# Move to your PATH
sudo mv asp-eks /usr/local/bin/
```

### Quick One-Line Installation

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
2. Export it: `export GITLAB_TOKEN=your_token_here`

### Manual Installation (Build from Source)

```bash
git clone https://github.com/eimarfandino/asp-eks.git
cd asp-eks
go build -o asp-eks
sudo mv asp-eks /usr/local/bin/
```

## 🍺 Homebrew Installation (Recommended)

```bash
brew tap eimarfandino/tap https://github.com/eimarfandino/homebrew-tap.git
brew install eimarfandino/tap/asp-eks
```

To upgrade:
```bash
brew upgrade eimarfandino/tap/asp-eks
```

Verify:
```bash
asp-eks --help
```

## Global Flags

- `-h, --help`: Display help information for asp-eks

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
