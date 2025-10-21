#!/usr/bin/sh

# Tempest
# Copyright (c) 2024, 2025 Sandstorm Development Team and contributors
# All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Status codes:
#  1 - SHA-256 value of the downloaded Go release did not match the expected value.
#  2 - Downloaded Go release could not be read during SHA-256 check.
#  3 - Go release path exists in download cache directory but is not a file.
#  4 - Prerequisite printf is missing.
#  5 - Prerequisite curl is missing.
#  6 - Prerequisite git is missing.
#  7 - Prerequisite gunzip is missing.
#  8 - Prerequisite make is missing.
#  9 - Prerequisite mkdir is missing.
# 10 - Prerequisite mv is missing.
# 11 - Prerequisite sha256sum or shasum is missing.
# 12 - Prerequisite rm is missing.
# 13 - Prerequisite sleep is missing.
# 14 - Prerequisite tar is missing.
# 15 - Failed to extract Go from Go release archive.
# 16 - Existing Go installation detected.
# 17 - Failed to remove the (created) SHA256SUMS file.
# 18 - Failed to download the Go release.

# User settings
[ -z "${DOWNLOAD_CACHE_DIR}" ] && DOWNLOAD_CACHE_DIR="${HOME}/.cache/tempest-build-tool/downloads"
[ -z "${DOWNLOAD_USER_AGENT}" ] && DOWNLOAD_USER_AGENT="tempest-bootstrap-build-tool"

script_dir="$(cd "$(dirname "$0")" && pwd)"
toolchain_dir="$(cd "${script_dir}"/.. && pwd)/toolchain"

go_version="1.25.3"
go_destination_file="go${go_version}.linux-amd64.tar.gz"
go_download_url="https://go.dev/dl/${go_destination_file}"
go_expected_sha256="0335f314b6e7bfe08c3d0cfaa7c19db961b7b99fb20be62b0a826c992ad14e0f"
go_downloaded_file="${DOWNLOAD_CACHE_DIR}/${go_destination_file}"
go_install_dir="${toolchain_dir}/go-${go_version}"
go_executable_file="go-${go_version}/bin/go"
toolchain_toml="${toolchain_dir}/toolchain.toml"

check_for_existing_installation() {
	install_dir="$1"
	if [ -d "$install_dir" ]; then
		fail 16 "Existing Go installation found at \"${install_dir}\""
	fi
}

# Ensure that the system has the commands necessary to run this script.
check_for_prerequisites() {
	if ! command -v printf >/dev/null 2>/dev/null; then
		# There is no point to use fail, which requires printf
		quit 4
	elif ! command -v curl >/dev/null 2>/dev/null; then
		fail 5 "The curl command, required to use this script, is not found."
	elif ! command -v git >/dev/null 2>/dev/null; then
		fail 6 "The git command, required to use this script, is not found."
	elif ! command -v gunzip >/dev/null 2>/dev/null; then
		fail 7 "The gunzip command, required to use this script, is not found."
	elif ! command -v make >/dev/null 2>/dev/null; then
		fail 8 "The make command, required to use this script, is not found."
	elif ! command -v mkdir >/dev/null 2>/dev/null; then
		fail 9 "The mkdir command, required to use this script, is not found."
	elif ! command -v mv >/dev/null 2>/dev/null; then
		fail 10 "The mv command, required to use this script, is not found."
	# sha256sum or shasum are checked at the call site
	elif ! command -v rm >/dev/null 2>/dev/null; then
		fail 12 "The rm command, required to use this script, is not found."
	elif ! command -v sleep >/dev/null 2>/dev/null; then
		fail 13 "The sleep command, required to use this script, is not found."
	elif ! command -v tar >/dev/null 2>/dev/null; then
		fail 14 "The tar command, required to use this script, is not found."
	fi
}

# Create the download cache directory if it does not exist.
create_download_cache_dir() {
	mkdir --parents "${DOWNLOAD_CACHE_DIR}"
}

# Create the toolchain.toml file if it does not exist.
create_toolchain_toml() {
	toolchain_toml="$1"
	go_executable="$2"
	go_version="$3"
	if [ ! -f "${toolchain_toml}" ]; then
		printf '[go]\n  Executable = "%s"\n  Version = "%s"\n' "${go_executable}" "${go_version}" >"${toolchain_toml}"
	fi
}

# Download Go from go.dev.
download_go() {
	download_url="$1"
	download_to_file="$2"
	if [ ! -f "${download_to_file}" ]; then
		if [ -e "${download_to_file}" ]; then
			fail 3 "Go release path exists but is not a normal file."
		fi
		printf 'Downloading %s' "${download_url}"
		retryable_curl "${download_url}" "${download_to_file}"
	fi
	if [ ! -f "${download_to_file}" ]; then
		fail 18 "Failed to download \"${download_to_file}\" from \"${download_url}\"."
	fi
	# Continue to SHA-256 check
}

# Extract the downloaded Go archive to the destination.
extract_go() {
	downloaded_file="$1"
	destination_path="$2"
	mkdir --parents "${destination_path}"
	# Using short options with tar for macOS compatibility
	if ! gunzip --stdout "${downloaded_file}" | tar -C "${destination_path}" -x; then
		fail 15 "Failed to extract \"${downloaded_file}\" to \"${destination_path}\"."
	fi
	# Go gives us ${destination_path}/go/...
	# Move the ... to ${destination_path}
	# (This is easier than trying to deal with transforming file names
	# during extraction.
	mv "${destination_path}"/go/* "${destination_path}"
	rmdir "${destination_path}/go"
}

# Print an error to stderr and exit
fail() {
	# Store the status code.
	status="$1"
	shift
	# Print the other arguments.
	printf '%s\n' "$*" >&2
	exit "$status"

}

# Ask the user a yes/no question.
prompt_yesno() {
	while true; do
		printf '%s' "$*"
		read -r input

		case "$input" in
		Y | y | YES | yes | Yes)
			return 0
			;;
		N | n | NO | no | No)
			return 1
			;;
		esac

		printf '%s' "Please answer \"yes\" or \"no\"."
	done
}

# Download a file, giving the user the option to retry on failure.
retryable_curl() {
	backoff_delay=1
	max_delay=5
	success="yes"
	[ -n "$3" ] && [ "$3" -ge 1 ] && backoff_delay=$3

	curl --user-agent "${DOWNLOAD_USER_AGENT}" --fail --location "$1" >"$2" || success="no"
	if [ "${success}" = "no" ]; then
		if prompt_yesno "Downloading $1 failed.  Retry? "; then
			wait_delay "$backoff_delay"
			backoff_delay=$((backoff_delay + 1))
			[ $backoff_delay -gt $max_delay ] && backoff_delay=$max_delay
			retryable_curl "$1" "$2" "$backoff_delay"
		else
			rm -f "$2"
		fi
	fi
}

# Verify that the SHA-256 value matches the expected value.
verify_sha256() {
	expected_sha256="$1"
	file_path="$2"

	# Build an SHA256SUMS file for the downloaded file.
	file_dir=$(dirname "${file_path}")
	file_name=$(basename "${file_path}")
	sha256sum_path="${file_dir}/SHA256SUMS"
	if [ -n "${file_dir}" ] && [ -e "$sha256sum_path" ]; then
		rm -f "$sha256sum_path"
	fi
	printf "%s *%s\n" "${expected_sha256}" "${file_name}" >>"${sha256sum_path}"

	# Check the SHA-256 value.
	pwd=$(pwd)
	cd "${file_dir}" || fail 2 "Failed to change to download directory."
	if command -v sha256sum >/dev/null 2>/dev/null; then
		sha256sum --check "${sha256sum_path}"
		sha256_rc=$?
	elif command -v shasum >/dev/null 2>/dev/null; then
		shasum --algorithm 256 --check "${sha256sum_path}"
		sha256_rc=$?
	else
		fail 11 "The sha256sum or shasum command, required to use this script, is not found."
	fi
	if [ "$sha256_rc" -ne 0 ]; then
		fail 1 "Failed to verify the SHA-256 value for the file ${file_path}."
	fi
	rm -f "$sha256sum_path" || fail 17 "Failed to remove (created) SHA256SUMS file."
	cd "${pwd}" || fail 2 "Failed to return to previous directory."
}

# Wait
wait_delay() {
	delay="$1"
	units="second"
	[ "$delay" -ne 1 ] && units="seconds"
	printf 'Waiting %d %s...' "${delay}" "${units}"
	sleep "$delay"
}

#trap cleanup HUP INT QUIT ABRT
check_for_prerequisites
check_for_existing_installation "${go_install_dir}"
create_download_cache_dir
download_go "${go_download_url}" "${go_downloaded_file}"
verify_sha256 "${go_expected_sha256}" "${go_downloaded_file}"
extract_go "${go_downloaded_file}" "${go_install_dir}"
create_toolchain_toml "${toolchain_toml}" "${go_executable_file}" "${go_version}"
