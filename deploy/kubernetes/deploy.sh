#!/bin/bash
set -euo pipefail

# --- Configuration ---
GITHUB_ORG="BytePeaks"
GITHUB_REPO="valiant"
GITHUB_BRANCH="release"
K8S_PATH="deploy/kubernetes"

BASE_URL="https://raw.githubusercontent.com/${GITHUB_ORG}/${GITHUB_REPO}/${GITHUB_BRANCH}/${K8S_PATH}"
KUSTOMIZATION_FILE="kustomization.yaml"

# --- Functions ---
log_info() {
    echo -e "\033[0;32m[INFO]\033[0m $1"
}

log_warn() {
    echo -e "\033[0;33m[WARN]\033[0m $1"
}

log_error() {
    echo -e "\033[0;31m[ERROR]\033[0m $1"
    exit 1
}

# --- Pre-requisites ---
if ! command -v kubectl &> /dev/null
then
    log_error "kubectl is not installed. Please install it to proceed."
fi

if ! command -v curl &> /dev/null
then
    log_error "curl is not installed. Please install it to proceed."
fi

# --- Main Deployment Logic ---
log_info "Starting Valiant Kubernetes deployment..."

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
log_info "Created temporary directory: ${TEMP_DIR}"

# Ensure cleanup on exit
cleanup() {
    log_info "Cleaning up temporary directory: ${TEMP_DIR}"
    rm -rf "${TEMP_DIR}"
}
trap cleanup EXIT

log_info "Downloading ${KUSTOMIZATION_FILE} from ${BASE_URL}/${KUSTOMIZATION_FILE}..."
if ! curl -sSL "${BASE_URL}/${KUSTOMIZATION_FILE}" -o "${TEMP_DIR}/${KUSTOMIZATION_FILE}"; then
    log_error "Failed to download ${KUSTOMIZATION_FILE}. Please check the URL and your internet connection."
fi

log_info "Applying Kubernetes resources using kustomization..."
# Using -f for file path explicitly, and --wait for completion
if ! kubectl apply -k "${TEMP_DIR}"; then
    log_error "Failed to apply Kubernetes resources. Check kubectl output above for details."
fi

log_info "Valiant has been successfully deployed to your Kubernetes cluster."
log_info "The namespace 'valiant' has been created along with its resources."
log_info "You can check the deployment status using: kubectl get all -n valiant"
log_info "To remove Valiant, run: kubectl delete namespace valiant"
log_info "Deployment script finished."
