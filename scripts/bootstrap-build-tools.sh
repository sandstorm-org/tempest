#!/usr/bin/sh

# Tempest
# Copyright (c) 2024 Sandstorm Development Team and contributors
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

#script_dir="$( cd "$( dirname "$0" )" && pwd )"
#build_tools_dir="${script_dir}/../build-tools"
[ -z "${download_user_agent}" ] && download_user_agent="tempest-bootstrap-build-tools"

go_destination_file="go1.23.3.linux-amd64.tar.gz"
go_download_url="https://go.dev/dl/${go_destination_file}"
go_expected_sha256="a0afb9744c00648bafb1b90b4aba5bdb86f424f02f9275399ce0c20b93a2c3a8"
work_dir="$(mktemp -d ./bootstrap-build-tools.XXXXXXXXXX)"
go_downloaded_file="${work_dir}/${go_destination_file}"

# Clean up after the script
cleanup() {
	rm -rf "${work_dir}"
}

# Print an error to stderr and exit
fail() {
	# Store the status code.
	status=$1
	shift
	# Print the other arguments.
	printf '%s\n' "$*" >&2
	exit "$status"

}

# Download Go from go.dev.
download_go() {
	download_url="$1"
	download_to_file="$2"
	echo "DOWNLOADING ${download_url}"
	retryable_curl "${download_url}" "${download_to_file}"
}

# Ask the user a yes/no question.
prompt_yesno() {
	while true; do
		input

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

		echo "Please answer \"yes\" or \"no\"."
	done
}

# Download a file, giving the user the option to retry on failure.
retryable_curl() {
	backoff_delay=1
	success="yes"
	[ -n "$3" ] && [ "$3" -ge 1 ] && backoff_delay=$3

	curl --user-agent "${download_user_agent}" --fail --location "$1" >"$2" || success="no"
	if [ "${success}" = "no" ]; then
		if prompt_yesno "Downloading $1 failed.  Retry? " "yes"; then
			wait_delay "$backoff_delay"
			backoff_delay=$(("$backoff_delay" + 1))
			retryable_curl "$1" "$2" "$backoff_delay"
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
	sha256sum_path=$(mktemp)
	printf "%s *%s\n" "${expected_sha256}" "${file_name}" >"${sha256sum_path}"
	cat "${sha256sum_path}"

	# Check the SHA-256 value.
	pwd=$(pwd)
	cd "${file_dir}" || fail 2 "Failed to change to download directory"
	if command -v sha256sum >/dev/null 2>/dev/null; then
		sha256sum --check "${sha256sum_path}"
		sha256_rc=$?
	elif command -v shasum >/dev/null 2>/dev/null; then
		shasum --algorithm 256 --check "${sha256sum_path}"
		sha256_rc=$?
	fi
	cd "${pwd}" || fail 2 "Failed to return to previous directory"

	if [ "$sha256_rc" -ne 0 ]; then
		rm "${sha256sum_path}"
		fail 1 "Failed to verify the SHA-256 value for the file ${file_name}"
	fi
	rm "${sha256sum_path}"
}

# Wait
wait_delay() {
	delay="$1"
	units="second"
	[ "$delay" -ne 1 ] && units="seconds"
	echo "Waiting ${delay} ${units}..."
	sleep "$delay"
}

trap cleanup HUP INT QUIT ABRT
download_go "${go_download_url}" "${go_downloaded_file}"
verify_sha256 "${go_expected_sha256}" "${go_downloaded_file}"
