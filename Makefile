REMOTE_PACKAGES := remote_packages.txt
DEFAULT_FILE := packages_id.txt
TEMP_FILE := .temp_packages_$(shell date +%s)

# Plain text output functions (no color formatting)
define print_red
	@printf 'ERROR: %s\n' "$(1)" >&2
endef

define print_green
	@printf 'SUCCESS: %s\n' "$(1)" >&2
endef

define print_yellow
	@printf 'WARNING: %s\n' "$(1)" >&2
endef

define print_blue
	@printf 'INFO: %s\n' "$(1)" >&2
endef

# Helper function to check if command exists
define check_command
	@which $(1) >/dev/null 2>&1 || (printf 'ERROR: %s not found. Please install it.\n' "$(1)" >&2 && exit 1)
endef

# Check dependencies
.PHONY: check-deps
check-deps:
	$(call print_blue,Checking required dependencies...)
	$(call check_command,curl)
	$(call check_command,jq)
	$(call check_command,grep)
	$(call check_command,sed)
	$(call print_green,All dependencies are satisfied!)

# Internal target to update a specific file
.PHONY: update-file-internal
update-file-internal:
	@if [ -z "$(TARGET_FILE)" ]; then \
		printf 'ERROR: TARGET_FILE not specified\n' >&2; \
		exit 1; \
	fi; \
	FILE_TO_PROCESS="$(TARGET_FILE)"; \
	if [ ! -f "$$FILE_TO_PROCESS" ]; then \
		printf 'ERROR: File not found: %s\n' "$$FILE_TO_PROCESS" >&2; \
		exit 1; \
	fi; \
	if [ ! -r "$$FILE_TO_PROCESS" ]; then \
		printf 'ERROR: Cannot read file: %s\n' "$$FILE_TO_PROCESS" >&2; \
		exit 1; \
	fi; \
	if [ ! -w "$$FILE_TO_PROCESS" ]; then \
		printf 'ERROR: Cannot write to file: %s\n' "$$FILE_TO_PROCESS" >&2; \
		exit 1; \
	fi; \
	printf 'INFO: Processing file: %s\n' "$$FILE_TO_PROCESS" >&2; \
	UPDATED=false; \
	TEMP_UPDATE_FILE=".temp_update_$$(basename "$$FILE_TO_PROCESS")_$$(date +%s)"; \
	> "$$TEMP_UPDATE_FILE"; \
	while IFS= read -r line || [ -n "$$line" ]; do \
		if echo "$$line" | grep -qE '^\s*#|^\s*$$'; then \
			echo "$$line" >> "$$TEMP_UPDATE_FILE"; \
		elif echo "$$line" | grep -qE '^\s*https://github\.com/.+/releases/download/.+\.pkg\.tar\.zst'; then \
			NEW_LINE=$$($(MAKE) -s process-github-line LINE="$$line" 2>/dev/null || echo "$$line"); \
			if [ "$$line" != "$$NEW_LINE" ]; then \
				UPDATED=true; \
			fi; \
			echo "$$NEW_LINE" >> "$$TEMP_UPDATE_FILE"; \
		elif echo "$$line" | grep -qE '^\s*https://gitlab\.com/.+/-/releases/.+/downloads/.+\.pkg\.tar\.zst'; then \
			NEW_LINE=$$($(MAKE) -s process-gitlab-line LINE="$$line" 2>/dev/null || echo "$$line"); \
			if [ "$$line" != "$$NEW_LINE" ]; then \
				UPDATED=true; \
			fi; \
			echo "$$NEW_LINE" >> "$$TEMP_UPDATE_FILE"; \
		else \
			echo "$$line" >> "$$TEMP_UPDATE_FILE"; \
		fi; \
	done < "$$FILE_TO_PROCESS"; \
	if [ "$$UPDATED" = "true" ]; then \
		mv "$$TEMP_UPDATE_FILE" "$$FILE_TO_PROCESS"; \
		printf 'SUCCESS: File updated: %s\n' "$$FILE_TO_PROCESS" >&2; \
	else \
		rm -f "$$TEMP_UPDATE_FILE"; \
		printf 'INFO: No updates needed for %s\n' "$$FILE_TO_PROCESS" >&2; \
	fi

# Main update target
.PHONY: update
update: check-deps
	@FILE_TO_PROCESS="$${FILE:-$(DEFAULT_FILE)}"; \
	$(MAKE) update-file-internal TARGET_FILE="$$FILE_TO_PROCESS"

# Update remote packages specifically
.PHONY: update-remote
update-remote: check-deps
	@$(MAKE) update-file-internal TARGET_FILE="$(REMOTE_PACKAGES)"

# Update both files
.PHONY: update-all
update-all: check-deps
	@$(MAKE) update-file-internal TARGET_FILE="$(DEFAULT_FILE)"
	@$(MAKE) update-file-internal TARGET_FILE="$(REMOTE_PACKAGES)"

# Process GitHub line (internal target)
.PHONY: process-github-line
process-github-line:
	@if [ -z "$(LINE)" ]; then exit 1; fi; \
	CLEAN_LINE=$$(echo "$(LINE)" | sed 's/^\s*//;s/\s*$$//'); \
	if ! echo "$$CLEAN_LINE" | grep -qE '^https://github\.com/[^/]+/[^/]+/releases/download/[^/]+/[^/]+\.pkg\.tar\.zst(\?[[:alnum:]._%=-]*)?$$'; then \
		echo "$(LINE)"; \
		exit 0; \
	fi; \
	OWNER=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://github\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\1|'); \
	REPO=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://github\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\2|'); \
	CURRENT_TAG=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://github\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\3|'); \
	FILENAME=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://github\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\4|'); \
	QUERY_PARAMS=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://github\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\5|'); \
	API_URL="https://api.github.com/repos/$$OWNER/$$REPO/releases/latest"; \
	printf 'INFO: Fetching latest release for %s/%s (current: %s)\n' "$$OWNER" "$$REPO" "$$CURRENT_TAG" >&2; \
	RESPONSE=$$(curl -s -w "HTTPSTATUS:%{http_code}" -H "Accept: application/vnd.github.v3+json" "$$API_URL"); \
	HTTP_STATUS=$$(echo "$$RESPONSE" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2); \
	JSON_RESPONSE=$$(echo "$$RESPONSE" | sed -E 's/HTTPSTATUS:[0-9]*$$//'); \
	case "$$HTTP_STATUS" in \
		200) \
			LATEST_TAG=$$(echo "$$JSON_RESPONSE" | jq -r '.tag_name // empty'); \
			if [ -z "$$LATEST_TAG" ] || [ "$$LATEST_TAG" = "null" ]; then \
				printf 'WARNING: No tag_name found in response for %s/%s\n' "$$OWNER" "$$REPO" >&2; \
				echo "$(LINE)"; \
				exit 0; \
			fi; \
			if [ "$$CURRENT_TAG" = "$$LATEST_TAG" ]; then \
				printf 'INFO: Already up to date: %s/%s (%s)\n' "$$OWNER" "$$REPO" "$$CURRENT_TAG" >&2; \
				echo "$(LINE)"; \
			else \
				FILENAME_PATTERN=$$(echo "$$FILENAME" | sed -E 's/v?[0-9]+(\.[0-9]+)*(-[0-9]+)?/[^-]*[0-9]+(\\.[0-9]+)*(-[0-9]+)?/g'); \
				MATCHING_ASSET=$$(echo "$$JSON_RESPONSE" | jq -r --arg pattern "$$FILENAME_PATTERN" '.assets[]? | select(.name | test($$pattern)) | .name' | head -n1); \
				if [ -n "$$MATCHING_ASSET" ] && [ "$$MATCHING_ASSET" != "null" ]; then \
					NEW_URL="https://github.com/$$OWNER/$$REPO/releases/download/$$LATEST_TAG/$$MATCHING_ASSET$$QUERY_PARAMS"; \
					printf 'SUCCESS: Updated %s/%s: %s -> %s\n' "$$OWNER" "$$REPO" "$$CURRENT_TAG" "$$LATEST_TAG" >&2; \
					echo "$$NEW_URL"; \
				else \
					printf 'WARNING: No matching asset found for pattern "%s" in %s/%s:%s\n' "$$FILENAME_PATTERN" "$$OWNER" "$$REPO" "$$LATEST_TAG" >&2; \
					printf 'INFO: Available assets: %s\n' "$$(echo "$$JSON_RESPONSE" | jq -r '.assets[]? | .name' | tr '\n' ' ')" >&2; \
					echo "$(LINE)"; \
				fi; \
			fi; \
			;; \
		403) \
			ERROR_MSG=$$(echo "$$JSON_RESPONSE" | jq -r '.message // "Unknown error"'); \
			printf 'WARNING: GitHub API error for %s/%s: %s\n' "$$OWNER" "$$REPO" "$$ERROR_MSG" >&2; \
			echo "$(LINE)"; \
			;; \
		404) \
			printf 'WARNING: Repository %s/%s not found or has no releases\n' "$$OWNER" "$$REPO" >&2; \
			echo "$(LINE)"; \
			;; \
		*) \
			ERROR_MSG=$$(echo "$$JSON_RESPONSE" | jq -r '.message // "Unknown error"' 2>/dev/null || echo "Unknown error"); \
			printf 'ERROR: GitHub API returned status %s for %s/%s: %s\n' "$$HTTP_STATUS" "$$OWNER" "$$REPO" "$$ERROR_MSG" >&2; \
			echo "$(LINE)"; \
			;; \
	esac

# Process GitLab line (internal target)  
.PHONY: process-gitlab-line
process-gitlab-line:
	@if [ -z "$(LINE)" ]; then exit 1; fi; \
	CLEAN_LINE=$$(echo "$(LINE)" | sed 's/^\s*//;s/\s*$$//'); \
	if ! echo "$$CLEAN_LINE" | grep -qE '^https://gitlab\.com/[^/]+/[^/]+/-/releases/[^/]+/downloads/[^/]+\.pkg\.tar\.zst(\?[[:alnum:]._%=-]*)?$$'; then \
		echo "$(LINE)"; \
		exit 0; \
	fi; \
	NAMESPACE=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://gitlab\.com/([^/]+)/([^/]+)/-/releases/([^/]+)/downloads/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\1|'); \
	PROJECT=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://gitlab\.com/([^/]+)/([^/]+)/-/releases/([^/]+)/downloads/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\2|'); \
	CURRENT_TAG=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://gitlab\.com/([^/]+)/([^/]+)/-/releases/([^/]+)/downloads/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\3|'); \
	FILENAME=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://gitlab\.com/([^/]+)/([^/]+)/-/releases/([^/]+)/downloads/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\4|'); \
	QUERY_PARAMS=$$(echo "$$CLEAN_LINE" | sed -E 's|^https://gitlab\.com/([^/]+)/([^/]+)/-/releases/([^/]+)/downloads/([^/]+\.pkg\.tar\.zst)(\?.*)?$$|\5|'); \
	PROJECT_PATH="$$NAMESPACE%2F$$PROJECT"; \
	PROJECT_API_URL="https://gitlab.com/api/v4/projects/$$PROJECT_PATH"; \
	CURL_OPTS=""; \
	if [ -n "$$GITLAB_TOKEN" ]; then \
		CURL_OPTS="-H \"PRIVATE-TOKEN: $$GITLAB_TOKEN\""; \
	fi; \
	printf 'INFO: Resolving GitLab project ID for %s/%s (current: %s)\n' "$$NAMESPACE" "$$PROJECT" "$$CURRENT_TAG" >&2; \
	PROJECT_RESPONSE=$$(eval curl -s -w "\"HTTPSTATUS:%{http_code}\"" $$CURL_OPTS "\"$$PROJECT_API_URL\""); \
	PROJECT_HTTP_STATUS=$$(echo "$$PROJECT_RESPONSE" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2); \
	PROJECT_JSON=$$(echo "$$PROJECT_RESPONSE" | sed -E 's/HTTPSTATUS:[0-9]*$$//'); \
	case "$$PROJECT_HTTP_STATUS" in \
		200) \
			PROJECT_ID=$$(echo "$$PROJECT_JSON" | jq -r '.id // empty'); \
			if [ -z "$$PROJECT_ID" ] || [ "$$PROJECT_ID" = "null" ]; then \
				printf 'ERROR: Could not get project ID for %s/%s\n' "$$NAMESPACE" "$$PROJECT" >&2; \
				echo "$(LINE)"; \
				exit 0; \
			fi; \
			RELEASES_API_URL="https://gitlab.com/api/v4/projects/$$PROJECT_ID/releases"; \
			printf 'INFO: Fetching latest release for GitLab project ID %s\n' "$$PROJECT_ID" >&2; \
			RELEASES_RESPONSE=$$(eval curl -s -w "\"HTTPSTATUS:%{http_code}\"" $$CURL_OPTS "\"$$RELEASES_API_URL\""); \
			RELEASES_HTTP_STATUS=$$(echo "$$RELEASES_RESPONSE" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2); \
			RELEASES_JSON=$$(echo "$$RELEASES_RESPONSE" | sed -E 's/HTTPSTATUS:[0-9]*$$//'); \
			case "$$RELEASES_HTTP_STATUS" in \
				200) \
					LATEST_RELEASE=$$(echo "$$RELEASES_JSON" | jq -r '.[0] // empty'); \
					if [ -z "$$LATEST_RELEASE" ] || [ "$$LATEST_RELEASE" = "null" ]; then \
						printf 'WARNING: No releases found for %s/%s\n' "$$NAMESPACE" "$$PROJECT" >&2; \
						echo "$(LINE)"; \
					else \
						LATEST_TAG=$$(echo "$$LATEST_RELEASE" | jq -r '.tag_name // empty'); \
						if [ -z "$$LATEST_TAG" ] || [ "$$LATEST_TAG" = "null" ]; then \
							printf 'WARNING: No tag_name found in latest release for %s/%s\n' "$$NAMESPACE" "$$PROJECT" >&2; \
							echo "$(LINE)"; \
							exit 0; \
						fi; \
						if [ "$$CURRENT_TAG" = "$$LATEST_TAG" ]; then \
							printf 'INFO: Already up to date: %s/%s (%s)\n' "$$NAMESPACE" "$$PROJECT" "$$CURRENT_TAG" >&2; \
							echo "$(LINE)"; \
						else \
							FILENAME_PATTERN=$$(echo "$$FILENAME" | sed -E 's/v?[0-9]+(\.[0-9]+)*(-[0-9]+)?/[^-]*[0-9]+(\\.[0-9]+)*(-[0-9]+)?/g'); \
							MATCHING_ASSET_URL=$$(echo "$$LATEST_RELEASE" | jq -r --arg pattern "$$FILENAME_PATTERN" '.assets.links[]? | select(.name | test($$pattern)) | .direct_asset_url // .url' | head -n1); \
							MATCHING_ASSET_NAME=$$(echo "$$LATEST_RELEASE" | jq -r --arg pattern "$$FILENAME_PATTERN" '.assets.links[]? | select(.name | test($$pattern)) | .name' | head -n1); \
							if [ -z "$$MATCHING_ASSET_URL" ] || [ "$$MATCHING_ASSET_URL" = "null" ]; then \
								if [ -n "$$MATCHING_ASSET_NAME" ] && [ "$$MATCHING_ASSET_NAME" != "null" ]; then \
									MATCHING_ASSET_URL="https://gitlab.com/$$NAMESPACE/$$PROJECT/-/releases/$$LATEST_TAG/downloads/$$MATCHING_ASSET_NAME$$QUERY_PARAMS"; \
								fi; \
							fi; \
							if [ -n "$$MATCHING_ASSET_URL" ] && [ "$$MATCHING_ASSET_URL" != "null" ]; then \
								printf 'SUCCESS: Updated %s/%s: %s -> %s\n' "$$NAMESPACE" "$$PROJECT" "$$CURRENT_TAG" "$$LATEST_TAG" >&2; \
								echo "$$MATCHING_ASSET_URL"; \
							else \
								printf 'WARNING: No matching asset found for pattern "%s" in %s/%s:%s\n' "$$FILENAME_PATTERN" "$$NAMESPACE" "$$PROJECT" "$$LATEST_TAG" >&2; \
								printf 'INFO: Available assets: %s\n' "$$(echo "$$LATEST_RELEASE" | jq -r '.assets.links[]? | .name' | tr '\n' ' ')" >&2; \
								echo "$(LINE)"; \
							fi; \
						fi; \
					fi; \
					;; \
				401|403) \
					ERROR_MSG=$$(echo "$$RELEASES_JSON" | jq -r '.message // "Authentication/authorization failed"'); \
					printf 'WARNING: GitLab API error for %s/%s: %s\n' "$$NAMESPACE" "$$PROJECT" "$$ERROR_MSG" >&2; \
					echo "$(LINE)"; \
					;; \
				404) \
					printf 'WARNING: No releases found for GitLab project %s/%s\n' "$$NAMESPACE" "$$PROJECT" >&2; \
					echo "$(LINE)"; \
					;; \
				*) \
					ERROR_MSG=$$(echo "$$RELEASES_JSON" | jq -r '.message // "Unknown error"' 2>/dev/null || echo "Unknown error"); \
					printf 'ERROR: GitLab API returned status %s for project %s: %s\n' "$$RELEASES_HTTP_STATUS" "$$PROJECT_ID" "$$ERROR_MSG" >&2; \
					echo "$(LINE)"; \
					;; \
			esac; \
			;; \
		401|403) \
			ERROR_MSG=$$(echo "$$PROJECT_JSON" | jq -r '.message // "Authentication/authorization failed"'); \
			printf 'WARNING: GitLab API error for %s/%s: %s\n' "$$NAMESPACE" "$$PROJECT" "$$ERROR_MSG" >&2; \
			echo "$(LINE)"; \
			;; \
		404) \
			printf 'WARNING: GitLab project %s/%s not found\n' "$$NAMESPACE" "$$PROJECT" >&2; \
			echo "$(LINE)"; \
			;; \
		*) \
			ERROR_MSG=$$(echo "$$PROJECT_JSON" | jq -r '.message // "Unknown error"' 2>/dev/null || echo "Unknown error"); \
			printf 'ERROR: GitLab API returned status %s for %s/%s: %s\n' "$$PROJECT_HTTP_STATUS" "$$NAMESPACE" "$$PROJECT" "$$ERROR_MSG" >&2; \
			echo "$(LINE)"; \
			;; \
	esac

# Target to sort packages file
.PHONY: sort
sort:
	@FILE_TO_PROCESS="$${FILE:-$(DEFAULT_FILE)}"; \
	if [ ! -f "$$FILE_TO_PROCESS" ]; then \
		printf 'ERROR: File not found: %s\n' "$$FILE_TO_PROCESS" >&2; \
		exit 1; \
	fi; \
	if [ ! -r "$$FILE_TO_PROCESS" ]; then \
		printf 'ERROR: Cannot read file: %s\n' "$$FILE_TO_PROCESS" >&2; \
		exit 1; \
	fi; \
	if [ ! -w "$$FILE_TO_PROCESS" ]; then \
		printf 'ERROR: Cannot write to file: %s\n' "$$FILE_TO_PROCESS" >&2; \
		exit 1; \
	fi; \
	printf 'INFO: Sorting packages in file: %s\n' "$$FILE_TO_PROCESS" >&2; \
	TEMP_SORT_FILE=$$(mktemp); \
	trap "rm -f '$$TEMP_SORT_FILE'" EXIT; \
	current_section_packages=""; \
	in_package_block=0; \
	while IFS= read -r line || [ -n "$$line" ]; do \
		if echo "$$line" | grep -qE '^[[:space:]]*#'; then \
			if [ $$in_package_block -eq 1 ] && [ -n "$$current_section_packages" ]; then \
				printf '%s' "$$current_section_packages" | \
				while IFS= read -r pkg_line; do \
					trimmed_pkg=$$(echo "$$pkg_line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
					if [ -n "$$trimmed_pkg" ]; then \
						printf '%s\t%s\n' "$$trimmed_pkg" "$$pkg_line"; \
					fi; \
				done | \
				sort -t$$'\t' -k1,1 | \
				awk -F$$'\t' '!seen[$$1]++ { print $$2 }' >> "$$TEMP_SORT_FILE"; \
				current_section_packages=""; \
			fi; \
			echo "$$line" >> "$$TEMP_SORT_FILE"; \
			in_package_block=0; \
		else \
			trimmed_content=$$(echo "$$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
			if [ -n "$$trimmed_content" ]; then \
				if [ -n "$$current_section_packages" ]; then \
					current_section_packages="$$current_section_packages"$$'\n'"$$line"; \
				else \
					current_section_packages="$$line"; \
				fi; \
				in_package_block=1; \
			else \
				if [ $$in_package_block -eq 1 ] && [ -n "$$current_section_packages" ]; then \
					printf '%s' "$$current_section_packages" | \
					while IFS= read -r pkg_line; do \
						trimmed_pkg=$$(echo "$$pkg_line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
						if [ -n "$$trimmed_pkg" ]; then \
							printf '%s\t%s\n' "$$trimmed_pkg" "$$pkg_line"; \
						fi; \
					done | \
					sort -t$$'\t' -k1,1 | \
					awk -F$$'\t' '!seen[$$1]++ { print $$2 }' >> "$$TEMP_SORT_FILE"; \
					current_section_packages=""; \
				fi; \
				echo "$$line" >> "$$TEMP_SORT_FILE"; \
				in_package_block=0; \
			fi; \
		fi; \
	done < "$$FILE_TO_PROCESS"; \
	if [ $$in_package_block -eq 1 ] && [ -n "$$current_section_packages" ]; then \
		printf '%s' "$$current_section_packages" | \
		while IFS= read -r pkg_line; do \
			trimmed_pkg=$$(echo "$$pkg_line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
			if [ -n "$$trimmed_pkg" ]; then \
				printf '%s\t%s\n' "$$trimmed_pkg" "$$pkg_line"; \
			fi; \
		done | \
		sort -t$$'\t' -k1,1 | \
		awk -F$$'\t' '!seen[$$1]++ { print $$2 }' >> "$$TEMP_SORT_FILE"; \
	fi; \
	if mv "$$TEMP_SORT_FILE" "$$FILE_TO_PROCESS"; then \
		printf 'SUCCESS: File %s sorted by sections and duplicates removed successfully\n' "$$FILE_TO_PROCESS" >&2; \
	else \
		printf 'ERROR: Failed to write sorted content to %s\n' "$$FILE_TO_PROCESS" >&2; \
		exit 1; \
	fi

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  update         - Update default packages file ($(DEFAULT_FILE))"
	@echo "  update-remote  - Update remote packages file ($(REMOTE_PACKAGES))"
	@echo "  update-all     - Update both packages files"
	@echo "  sort           - Sort packages by sections and remove duplicates"
	@echo "  check-deps     - Check if required dependencies are installed"
	@echo "  help           - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  FILE         - Override default file for 'update' target (default: $(DEFAULT_FILE))"
	@echo "  GITLAB_TOKEN - GitLab API token for private repositories"
	@echo ""
	@echo "Examples:"
	@echo "  make update                    # Update $(DEFAULT_FILE)"
	@echo "  make update-remote             # Update $(REMOTE_PACKAGES)"
	@echo "  make update-all                # Update both files"
	@echo "  make update FILE=my_file.txt   # Update custom file"
	@echo "  GITLAB_TOKEN=token make update-remote"
	@echo "  make sort FILE=$(REMOTE_PACKAGES)  # Sort remote packages"
