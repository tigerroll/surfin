# syntax=docker/dockerfile:1

# ==============================================================================
# Stage 1: go-builder - To install Go tools
# ==============================================================================
FROM --platform=linux/amd64 fedora:latest AS go-builder

# Install base dependencies for cloning and downloading, and for Go build
RUN dnf install -y git unzip curl gcc && dnf clean all

# Install goenv from git for version management
RUN git clone --depth=1 https://github.com/go-nv/goenv.git /opt/goenv

# Install Go and related tools using goenv
ENV GOENV_ROOT="/opt/goenv"
ENV PATH="$GOENV_ROOT/bin:$PATH"
RUN eval "$(goenv init -)" && \
    goenv install latest && \
    goenv global latest && \
    go install github.com/go-task/task/v3/cmd/task@latest && \
    go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest && \
    go install golang.org/x/tools/gopls@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    goenv rehash

# ==============================================================================
# Stage 2: python-builder - To build a specific Python version with pyenv
# ==============================================================================
FROM --platform=linux/amd64 fedora:latest AS python-builder

# Install git and all dependencies required to build Python from source
RUN dnf install -y \
    git \
    gcc \
    make \
    bzip2-devel \
    libffi-devel \
    openssl-devel \
    readline-devel \
    sqlite-devel \
    xz-devel \
    zlib-devel \
    python3-devel && \
    dnf clean all

# Clone the pyenv repository
RUN git clone --depth=1 https://github.com/pyenv/pyenv.git /opt/pyenv

# Use the pyenv command to install Python 3.11.9
ENV PYENV_ROOT="/opt/pyenv"
ENV PATH="$PYENV_ROOT/bin:$PATH"
RUN pyenv install 3.11.9 && \
    pyenv global 3.11.9

# Create a virtual environment and install aider-chat into it
RUN eval "$(pyenv init -)" && \
    python -m venv /opt/venv && \
    /opt/venv/bin/pip install --upgrade pip setuptools wheel && \
    /opt/venv/bin/pip install aider-chat

# ==============================================================================
# Stage 3: Final - The actual runtime environment
# ==============================================================================
FROM --platform=linux/amd64 fedora:latest

# --- Install Base System Packages and Google Cloud SDK ---

ARG dir="."
COPY ${dir}/entrypoint.sh /opt/entrypoint.sh
RUN chmod +x /opt/entrypoint.sh
COPY ${dir}/google-cloud-sdk.repo /etc/yum.repos.d/google-cloud-sdk.repo

# Install base package first
RUN dnf update -y && dnf install -y \
    mysql postgresql awscli libxcrypt-compat && dnf clean all
RUN bash -x /opt/entrypoint.sh install_package_groups
RUN bash -x /opt/entrypoint.sh install_packages
RUN bash -x /opt/entrypoint.sh install_google_cloud_sdk

# --- Copy Tools from Builder Stages ---

# Copy pyenv and the installed Python version from the python-builder stage
COPY --from=python-builder /opt/pyenv /opt/pyenv

# Copy the virtual environment containing aider-chat
COPY --from=python-builder /opt/venv /opt/venv

# Copy goenv from the go-builder stage
COPY --from=go-builder /opt/goenv /opt/goenv

# Copy Go binaries (like 'task') from the go-builder stage
COPY --from=go-builder /root/go /root/go

# --- Configure Environment Variables for All Tools ---

ENV LANG=ja_JP.UTF-8

# Set PYENV_ROOT for pyenv to work correctly
ENV PYENV_ROOT="/opt/pyenv"

# Set GOENV_ROOT for goenv to work correctly
ENV GOENV_ROOT="/opt/goenv"

# Set the PATH to include the venv, shims, pyenv bin, goenv shims, and goenv bin
ENV PATH="/opt/venv/bin:$PYENV_ROOT/shims:$PYENV_ROOT/bin:$GOENV_ROOT/shims:$GOENV_ROOT/bin:$PATH"

# Configure pyenv to initialize automatically for login shells for convenience
RUN echo 'eval "$(pyenv init -)"' >> /etc/profile.d/pyenv.sh

# Configure goenv to initialize automatically for login shells for convenience
RUN echo 'eval "$(goenv init -)"' >> /etc/profile.d/goenv.sh

WORKDIR /opt/go_project

CMD ["bash"]
