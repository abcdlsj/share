#!/bin/zsh

export CLIP_COPY="pbcopy"
export CLIP_PASTE="pbpaste"

function up {
    local clipboard_content=$(eval "$CLIP_PASTE")
    local url="https://wkclip.octoptree.xyz/"

    local response=$(curl -s -d "$clipboard_content" "$url")
    echo "$response"
}

function down {
    local url="https://wkclip.octoptree.xyz/"

    local response=$(curl -s "$url")
    echo "$response" | cat | eval "$CLIP_COPY"
    echo "Result downloaded to clipboard"
}