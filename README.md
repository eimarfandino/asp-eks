# asp-eks

Tool for moving to a different EKS cluster using AWS profiles

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
- `help`: Help about any command
- `list`: List available AWS profiles
- `use`: Use a specific AWS profile and set kubeconfig for an EKS cluster


## 🔧 Installation

### Using Homebrew (macOS)

```bash
# Add the tap repository
brew tap eimarfandino/tap

# Install asp-eks
brew install asp-eks
```
### Manual Installation

```bash
# Clone the repository
git clone https://github.com/eimarfandino/asp-eks.git

# Navigate to the directory
cd asp-eks

# Build the binary
go build -o asp-eks

# Move the binary to your PATH (optional)
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

Get help for a specific command:
```bash
asp-eks [command] --help
```

## Global Flags

- `-h, --help`: Display help information for asp-eks

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