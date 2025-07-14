FROM archlinux:latest

RUN pacman -Syu --noconfirm && \
    pacman -S --noconfirm curl jq base-devel git unzip rsync && \
    curl -fsSL https://bun.sh/install | bash

ENV PATH="/root/.bun/bin:${PATH}"
