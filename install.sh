#!/bin/bash

# asp-eks installation script
# Downloads and installs the latest asp-eks release based on OS and architecture

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GITLAB_PROJECT_ID="69875845"
BINARY_NAME="asp-eks"
INSTALL_DIR="/usr/local/bin"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        arm64|aarch64)
            echo "arm64"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            print_error "Supported architectures: x86_64 (Intel), arm64 (Apple Silicon)"
            exit 1
            ;;
    esac
}

# Function to detect OS
detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case $os in
        darwin)
            echo "darwin"
            ;;
        linux)
            echo "linux"
            ;;
        *)
            print_error "Unsupported operating system: $os"
            print_error "Supported OS: macOS (darwin), Linux"
            exit 1
            ;;
    esac
}

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    if ! command -v curl >/dev/null 2>&1; then
        print_error "curl is required but not installed. Please install curl first."
        exit 1
    fi
    
    if ! command -v unzip >/dev/null 2>&1; then
        print_error "unzip is required but not installed. Please install unzip first."
        exit 1
    fi
    
    print_success "Prerequisites check passed"
}

# Function to get latest release from GitLab
get_latest_release() {
    print_status "Getting latest release information..." >&2
    
    local release_url="https://gitlab.com/api/v4/projects/${GITLAB_PROJECT_ID}/releases"
    local tags_url="https://gitlab.com/api/v4/projects/${GITLAB_PROJECT_ID}/repository/tags"
    local latest_release
    
    # Try with authentication first if token is available
    if [ -n "$GITLAB_TOKEN" ]; then
        print_status "Using GitLab token for authentication" >&2
        
        # Try releases API first with token
        if command -v jq >/dev/null 2>&1; then
            latest_release=$(curl -s -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$release_url" 2>/dev/null | jq -r '.[0].tag_name' 2>/dev/null)
        fi
        
        # Try tags API if releases failed
        if [ -z "$latest_release" ] || [ "$latest_release" = "null" ]; then
            if command -v jq >/dev/null 2>&1; then
                latest_release=$(curl -s -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$tags_url" 2>/dev/null | jq -r '.[0].name' 2>/dev/null)
            else
                latest_release=$(curl -s -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$tags_url" 2>/dev/null | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
            fi
        fi
    else
        # Try without authentication (for public projects)
        print_status "No GitLab token provided, trying public access..." >&2
        
        if command -v jq >/dev/null 2>&1; then
            latest_release=$(curl -s "$release_url" 2>/dev/null | jq -r '.[0].tag_name' 2>/dev/null)
        fi
        
        # Try tags API if releases failed
        if [ -z "$latest_release" ] || [ "$latest_release" = "null" ]; then
            if command -v jq >/dev/null 2>&1; then
                latest_release=$(curl -s "$tags_url" 2>/dev/null | jq -r '.[0].name' 2>/dev/null)
            else
                latest_release=$(curl -s "$tags_url" 2>/dev/null | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
            fi
        fi
    fi
    
    # If still no release, provide helpful error
    if [ -z "$latest_release" ] || [ "$latest_release" = "null" ]; then
        print_error "Could not determine latest release version" >&2
        print_error "This could be because:" >&2
        print_error "1. The project requires authentication (set GITLAB_TOKEN)" >&2
        print_error "2. No releases/tags exist yet" >&2
        print_error "3. Network connectivity issues" >&2
        print_error "" >&2
        print_error "You can:" >&2
        print_error "1. Set GITLAB_TOKEN environment variable: export GITLAB_TOKEN=your_token" >&2
        print_error "2. Download manually from: https://github.com/eimarfandino/asp-eks/releases" >&2
        exit 1
    fi
    
    print_success "Latest release: $latest_release" >&2
    echo "$latest_release"
}

# Function to download using GitLab artifacts API
download_from_artifacts() {
    local os="$1"
    local arch="$2"
    local version="$3"
    local temp_dir="$4"
    
    if [ -z "$GITLAB_TOKEN" ]; then
        return 1
    fi
    
    print_status "Trying GitLab artifacts API..."
    
    # Get the latest successful package job
    local jobs_url="https://gitlab.com/api/v4/projects/${GITLAB_PROJECT_ID}/jobs?scope=success&per_page=10"
    local job_id
    
    if command -v jq >/dev/null 2>&1; then
        job_id=$(curl -s -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$jobs_url" | jq -r '.[] | select(.name == "package") | .id' | head -1)
    else
        print_warning "jq not available, cannot parse GitLab API response"
        return 1
    fi
    
    if [ -z "$job_id" ] || [ "$job_id" = "null" ]; then
        print_warning "Could not find package job"
        return 1
    fi
    
    print_status "Found package job: $job_id"
    
    # Download full artifacts
    local artifacts_url="https://gitlab.com/api/v4/projects/${GITLAB_PROJECT_ID}/jobs/${job_id}/artifacts"
    local zip_file="$temp_dir/artifacts.zip"
    
    if ! curl -fsSL -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$artifacts_url" -o "$zip_file"; then
        print_warning "Failed to download artifacts"
        return 1
    fi
    
    # Check if we got a proper zip file
    if ! file "$zip_file" | grep -q "Zip archive"; then
        print_warning "Downloaded file is not a zip archive"
        return 1
    fi
    
    # Extract the specific binary
    local binary_pattern="dist/${BINARY_NAME}_${version}_${os}_${arch}.zip"
    local binary_zip
    
    # List contents and find our file
    if ! unzip -l "$zip_file" 2>/dev/null | grep -q "$binary_pattern"; then
        print_warning "Could not find binary for ${os}_${arch} in artifacts"
        return 1
    fi
    
    binary_zip=$(unzip -l "$zip_file" | grep "$binary_pattern" | awk '{print $4}' | head -1)
    
    if [ -z "$binary_zip" ]; then
        print_warning "Could not find binary for ${os}_${arch} in artifacts"
        return 1
    fi
    
    print_status "Extracting: $binary_zip"
    
    # Extract the zip file containing our binary
    if ! unzip -j "$zip_file" "$binary_zip" -d "$temp_dir"; then
        print_warning "Failed to extract binary zip"
        return 1
    fi
    
    # Extract the binary from the nested zip
    local nested_zip="$temp_dir/$(basename "$binary_zip")"
    if ! unzip -j "$nested_zip" "*/$BINARY_NAME" -d "$temp_dir" 2>/dev/null; then
        if ! unzip -j "$nested_zip" "$BINARY_NAME" -d "$temp_dir" 2>/dev/null; then
            print_warning "Failed to extract binary from nested zip"
            return 1
        fi
    fi
    
    # Clean up
    rm -f "$zip_file" "$nested_zip"
    
    if [ -f "$temp_dir/$BINARY_NAME" ]; then
        return 0
    else
        return 1
    fi
}

# Function to download and install
download_and_install() {
    local os="$1"
    local arch="$2"
    local version="$3"
    
    # Create temporary directory
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT
    
    print_status "Downloading binary for ${os}/${arch}..."
    
    # Try GitLab artifacts API if token is available
    if download_from_artifacts "$os" "$arch" "$version" "$temp_dir"; then
        print_success "Successfully downloaded from GitLab artifacts"
    else
        # If artifacts method failed, provide manual instructions
        print_warning "Automatic download failed"
        print_error ""
        print_error "Please download manually:"
        print_error "1. Visit: https://github.com/eimarfandino/asp-eks/releases"
        print_error "2. Download the appropriate file for your system:"
        
        if [ "$os" = "darwin" ] && [ "$arch" = "arm64" ]; then
            print_error "   - macOS Apple Silicon (Darwin ARM64)"
        elif [ "$os" = "darwin" ] && [ "$arch" = "amd64" ]; then
            print_error "   - macOS Intel (Darwin AMD64)"
        elif [ "$os" = "linux" ] && [ "$arch" = "arm64" ]; then
            print_error "   - Linux ARM64"
        elif [ "$os" = "linux" ] && [ "$arch" = "amd64" ]; then
            print_error "   - Linux Intel (AMD64)"
        fi
        
        print_error "3. Extract the zip file and move the 'asp-eks' binary to $INSTALL_DIR"
        print_error "4. Make it executable: chmod +x $INSTALL_DIR/asp-eks"
        exit 1
    fi
    
    # Verify the binary exists
    if [ ! -f "$temp_dir/$BINARY_NAME" ]; then
        print_error "Binary not found after download"
        exit 1
    fi
    
    # Make binary executable
    chmod +x "$temp_dir/$BINARY_NAME"
    
    # Test the binary
    print_status "Testing the downloaded binary..."
    if ! "$temp_dir/$BINARY_NAME" --help >/dev/null 2>&1; then
        print_warning "Binary test failed, but continuing with installation..."
    fi
    
    # Install the binary
    print_status "Installing to $INSTALL_DIR..."
    
    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$temp_dir/$BINARY_NAME" "$INSTALL_DIR/"
    else
        print_status "Administrator privileges required for installation to $INSTALL_DIR"
        sudo mv "$temp_dir/$BINARY_NAME" "$INSTALL_DIR/"
    fi
    
    # Verify installation
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        local installed_version
        installed_version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        print_success "asp-eks installed successfully!"
        print_success "Version: $installed_version"
        print_success "You can now use 'asp-eks --help' to get started"
    else
        print_warning "Installation completed, but asp-eks is not in PATH"
        print_warning "You may need to restart your shell or add $INSTALL_DIR to your PATH"
    fi
}

# Main installation function
main() {
    echo
    print_status "asp-eks Installation Script"
    print_status "=========================="
    echo
    
    # Detect system
    local os arch version
    os=$(detect_os)
    arch=$(detect_arch)
    
    print_status "Detected system: ${os}/${arch}"
    if [ "$os" = "darwin" ]; then
        if [ "$arch" = "amd64" ]; then
            print_status "macOS Intel detected"
        else
            print_status "macOS Apple Silicon detected"
        fi
    fi
    
    check_prerequisites
    version=$(get_latest_release)
    download_and_install "$os" "$arch" "$version"
    
    echo
    print_success "Installation complete!"
    echo
}

# Run main function
main "$@"
