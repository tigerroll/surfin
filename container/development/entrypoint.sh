#!/usr/bin/env bash
set -e

function install_package_groups() {
  echo "Installing package groups..."
  # package group definitions.
  local package_groups+=("development-tools")

  # install package groups.
  dnf group install "${package_groups[@]}" -y
  return $?
}

function install_packages() {
  echo "Installing individual packages..."
  # package definitions.
  local packages+=("procps-ng") 
  local packages+=("wget")
  local packages+=("unzip")
	local packages+=("python3")
  local packages+=("python3-pip")
  local packages+=("iproute")
  local packages+=("util-linux")
  local packages+=("git")
  local packages+=("curl")
  local packages+=("file")
  local packages+=("nc")
  local packages+=("vim")
  local packages+=("jq")
  local packages+=("net-tools")
  local packages+=("bind-utils")
  local packages+=("socat")
  local packages+=("bash-completion")
  local packages+=("glibc-langpack-ja")
  local packages+=("awk")
  local packages+=("tree")
  local packages+=("yq")
  local packages+=("the_silver_searcher")
  local packages+=("htop")
  local packages+=("btop")
  local packages+=("nkf")

  # install packages.
  dnf install "${packages[@]}" -y && dnf clean all
  return $?
}

function install_google_cloud_sdk() {
  # install of packages.
	dnf install -y --nogpgcheck google-cloud-cli
	dnf clean all
  return $?
}

case ${1} in
  "install_package_groups") install_package_groups;; 
  "install_packages") install_packages;; 
  "install_google_cloud_sdk") install_google_cloud_sdk;; 
  *) exec "$@";; 
esac
