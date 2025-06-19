#!/bin/bash

# Copyright (C) 2025 CrystalNetwork Studio
#
# This file is part of the CrystalNetwork Studio, CrystalLinux Repository.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program. If not, see <https://www.gnu.org/licenses/>.

# --- Configuration ---
REPO_NAME="crystallinux"
SERVER_URL="https://crystalnetwork-studio.gitlab.io/linux/CrystalLinux/packages/gitlab-profile/\$arch"

# Configuration file paths
REPO_FILE_NAME="${REPO_NAME}.conf"
REPO_FILE_PATH="/etc/pacman.d/${REPO_FILE_NAME}"
PACMAN_CONF="/etc/pacman.conf"
INCLUDE_LINE="Include = ${REPO_FILE_PATH}"

# --- Helper Functions ---
print_success() {
    echo -e "\e[32m[SUCCESS]\e[0m $1"
}

print_error() {
    echo -e "\e[31m[ERROR]\e[0m $1" >&2
}

print_info() {
    echo -e "\e[34m[INFO]\e[0m $1"
}

# --- Main Functions ---

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (e.g., using sudo)."
        exit 1
    fi
}

check_distro() {
    print_info "Checking distribution compatibility..."
    if ! command -v pacman &> /dev/null; then
        print_error "Pacman command not found. This script requires an Arch Linux based distribution."
        exit 1
    fi

    local is_arch_based=false
    if [[ -f /etc/os-release ]]; then
        if grep -q -E '(^|\s|,)arch($|\s|,)' /etc/os-release /dev/null; then
          if grep -q 'ID_LIKE=' /etc/os-release /dev/null && grep -q 'ID_LIKE=.*arch.*' /etc/os-release /dev/null ; then
            is_arch_based=true
            print_info "Detected Arch-based distribution via /etc/os-release (ID_LIKE)."
          elif grep -q '^ID=arch$' /etc/os-release /dev/null ; then
            is_arch_based=true
            print_info "Detected Arch Linux via /etc/os-release (ID)."
          fi
        fi
    fi

    if [[ "$is_arch_based" == "false" ]] && [[ -f /etc/arch-release ]]; then
        is_arch_based=true
        print_info "Detected Arch Linux via /etc/arch-release."
    fi

    if [[ "$is_arch_based" == "false" ]]; then
        print_error "Distribution is not recognized as Arch Linux or Arch-based."
        print_error "This script modifies pacman configuration and is intended only for such systems."
        exit 1
    fi
    print_success "Distribution check passed."
}

install_repo() {
    print_info "Starting repository installation..."

    # Create the repository configuration file content.
    REPO_CONFIG="[${REPO_NAME}]\nSigLevel = Optional TrustAll\nServer = ${SERVER_URL}\n"

    print_info "Creating repository configuration file at ${REPO_FILE_PATH}..."
    echo -e "${REPO_CONFIG}" | sudo tee "${REPO_FILE_PATH}" > /dev/null
    if [[ $? -ne 0 ]]; then
        print_error "Failed to create repository configuration file."
        exit 1
    fi
    print_success "Repository configuration file created."

    print_info "Checking if '${INCLUDE_LINE}' exists in ${PACMAN_CONF}..."
    if grep -qF "${INCLUDE_LINE}" "${PACMAN_CONF}"; then
        print_info "Include line already exists in ${PACMAN_CONF}. No changes needed there."
    else
        print_info "Adding Include line to ${PACMAN_CONF}..."
        if sudo sh -c "echo '${INCLUDE_LINE}' >> '${PACMAN_CONF}'"; then
            print_success "Include line added to ${PACMAN_CONF}."
        else
            print_error "Failed to add Include line to ${PACMAN_CONF}."
            sudo rm -f "${REPO_FILE_PATH}"
            exit 1
        fi
    fi

    print_success "Repository '${REPO_NAME}' successfully configured."
    print_info "Please run 'sudo pacman -Syu' to synchronize databases."
}

uninstall_repo() {
    print_info "Starting repository uninstallation..."

    if [[ -f "${REPO_FILE_PATH}" ]]; then
        print_info "Removing repository configuration file ${REPO_FILE_PATH}..."
        if sudo rm -f "${REPO_FILE_PATH}"; then
            print_success "Repository configuration file removed."
        else
            print_error "Failed to remove repository configuration file. Please remove it manually."
        fi
    else
        print_info "Repository configuration file ${REPO_FILE_PATH} not found."
    fi

    print_info "Checking for Include line in ${PACMAN_CONF}..."
    if grep -qF "${INCLUDE_LINE}" "${PACMAN_CONF}"; then
        print_info "Removing Include line from ${PACMAN_CONF}..."
        if sudo sed -i "\#^${INCLUDE_LINE}\$#d" "${PACMAN_CONF}"; then
            print_success "Include line removed from ${PACMAN_CONF}."
        else
            print_error "Failed to remove Include line automatically. Please remove it manually from ${PACMAN_CONF}:"
            print_error "${INCLUDE_LINE}"
            exit 1
        fi
    else
        print_info "Include line not found in ${PACMAN_CONF}. No changes needed there."
    fi

    print_success "Repository '${REPO_NAME}' successfully uninstalled."
    print_info "You may want to run 'sudo pacman -Syu' to refresh."
}


# --- Script Execution ---

check_root
check_distro

if [[ "$1" == "--uninstall" ]]; then
    uninstall_repo
else
    install_repo
fi

exit 0