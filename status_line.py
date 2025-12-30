#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "python-dotenv",
#     "pygithub",
# ]
# ///

import json
import os
import sys
import subprocess
from pathlib import Path
from datetime import datetime

try:
    from dotenv import load_dotenv
    load_dotenv()
except ImportError:
    pass  # dotenv is optional

try:
    from github import Github
except ImportError:
    Github = None  # PyGithub is optional

# Configuration constants
LOG_BASE_DIR = Path.home() / ".claude" / "logs"


def log_status_line(input_data, status_line_output, github_status_info=None):
    """Log status line event to logs directory."""
    # Ensure logs directory exists
    LOG_BASE_DIR.mkdir(parents=True, exist_ok=True)
    log_file = LOG_BASE_DIR / 'status_line.json'

    # Read existing log data or initialize empty list
    if log_file.exists():
        with open(log_file, 'r') as f:
            try:
                log_data = json.load(f)
            except (json.JSONDecodeError, ValueError):
                log_data = []
    else:
        log_data = []

    # Create log entry with input data and generated output
    log_entry = {
        "timestamp": datetime.now().isoformat(),
        "input_data": input_data,
        "status_line_output": status_line_output
    }

    # Add GitHub status info if available
    if github_status_info:
        log_entry["github_status"] = github_status_info

    # Append the log entry
    log_data.append(log_entry)

    # Write back to file with formatting
    with open(log_file, 'w') as f:
        json.dump(log_data, f, indent=2)


def get_git_branch():
    """Get current git branch if in a git repository."""
    try:
        result = subprocess.run(
            ['git', 'rev-parse', '--abbrev-ref', 'HEAD'],
            capture_output=True,
            text=True,
            timeout=2
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except Exception:
        pass
    return None


def get_git_status():
    """Get git status indicators."""
    try:
        # Check if there are uncommitted changes
        result = subprocess.run(
            ['git', 'status', '--porcelain'],
            capture_output=True,
            text=True,
            timeout=2
        )
        if result.returncode == 0:
            changes = result.stdout.strip()
            if changes:
                lines = changes.split('\n')
                return f"±{len(lines)}"
    except Exception:
        pass
    return ""


def get_gh_token():
    """Get GitHub token from gh CLI authentication."""
    try:
        result = subprocess.run(
            ['gh', 'auth', 'token'],
            capture_output=True,
            text=True,
            timeout=2
        )
        if result.returncode == 0:
            token = result.stdout.strip()
            if token:
                return token
    except Exception:
        pass
    return None


def get_github_repo():
    """Parse GitHub owner/repo from git remote URL."""
    try:
        result = subprocess.run(
            ['git', 'remote', 'get-url', 'origin'],
            capture_output=True,
            text=True,
            timeout=2
        )
        if result.returncode == 0:
            url = result.stdout.strip()
            # Handle SSH format: git@github.com:owner/repo.git
            if url.startswith('git@github.com:'):
                repo_path = url.replace('git@github.com:', '').replace('.git', '')
                parts = repo_path.split('/')
                if len(parts) == 2:
                    return parts[0], parts[1]
            # Handle HTTPS format: https://github.com/owner/repo.git
            elif 'github.com/' in url:
                repo_path = url.split('github.com/')[-1].replace('.git', '')
                parts = repo_path.split('/')
                if len(parts) == 2:
                    return parts[0], parts[1]
    except Exception:
        pass
    return None, None


def get_github_build_status(owner, repo, branch, token):
    """Fetch latest build_and_test workflow status for branch.

    Returns:
        tuple: (status, error_message) where status is one of:
               'success', 'failure', 'pending', 'error', or None
    """
    if not Github or not owner or not repo or not branch:
        return None, "missing_data"

    if not token:
        return None, "no_auth"

    try:
        # Initialize GitHub client with timeout
        gh = Github(token, timeout=2)
        gh_repo = gh.get_repo(f"{owner}/{repo}")

        # Get workflows and find build_and_test
        workflows = gh_repo.get_workflows()
        build_workflow = None
        for workflow in workflows:
            if workflow.name == "build_and_test" or workflow.path.endswith("build_and_test.yml"):
                build_workflow = workflow
                break

        if not build_workflow:
            return None, "workflow_not_found"

        # Get latest workflow runs for the branch
        runs = build_workflow.get_runs(branch=branch)
        if runs.totalCount == 0:
            return None, "no_runs"

        # Get the most recent run
        latest_run = runs[0]

        # Map GitHub conclusion to our status
        if latest_run.status == "completed":
            if latest_run.conclusion == "success":
                return "success", None
            elif latest_run.conclusion in ["failure", "timed_out", "cancelled"]:
                return "failure", None
            else:
                return "error", f"unexpected_conclusion:{latest_run.conclusion}"
        elif latest_run.status in ["queued", "in_progress", "waiting"]:
            return "pending", None
        else:
            return "error", f"unexpected_status:{latest_run.status}"

    except Exception as e:
        return "error", str(e)[:50]  # Truncate error message


def generate_status_line(input_data):
    """Generate the status line based on input data."""
    parts = []
    github_status_info = None

    # Model display name
    model_info = input_data.get('model', {})
    model_name = model_info.get('display_name', 'Claude')
    parts.append(f"\033[36m[{model_name}]\033[0m")  # Cyan color

    # Current directory
    workspace = input_data.get('workspace', {})
    current_dir = workspace.get('current_dir', '')
    if current_dir:
        dir_name = os.path.basename(current_dir)
        parts.append(f"\033[34m📁 {dir_name}\033[0m")  # Blue color

    # Git branch and status
    git_branch = get_git_branch()
    if git_branch:
        git_status = get_git_status()
        git_info = f"🌿 {git_branch}"
        if git_status:
            git_info += f" {git_status}"
        parts.append(f"\033[32m{git_info}\033[0m")  # Green color

        # GitHub build status
        gh_token = get_gh_token()
        owner, repo = get_github_repo()
        if owner and repo:
            build_status, error_msg = get_github_build_status(owner, repo, git_branch, gh_token)
            github_status_info = {"status": build_status, "error": error_msg, "owner": owner, "repo": repo}

            # Map status to emoji
            status_emoji = {
                "success": "✅",
                "failure": "❌",
                "pending": "🔄",
                "error": "⚠️",
            }
            emoji = status_emoji.get(build_status, "⚠️")
            parts.append(f"\033[33m{emoji}\033[0m")  # Yellow color

    # Version info (optional, smaller)
    version = input_data.get('version', '')
    if version:
        parts.append(f"\033[90mv{version}\033[0m")  # Gray color

    return " | ".join(parts), github_status_info


def main():
    try:
        # Read JSON input from stdin
        input_data = json.loads(sys.stdin.read())

        # Generate status line
        status_line, github_status_info = generate_status_line(input_data)

        # Log the status line event
        log_status_line(input_data, status_line, github_status_info)

        # Output the status line (first line of stdout becomes the status line)
        print(status_line)

        # Success
        sys.exit(0)

    except json.JSONDecodeError:
        # Handle JSON decode errors gracefully - output basic status
        print("\033[31m[Claude] 📁 Unknown\033[0m")
        sys.exit(0)
    except Exception:
        # Handle any other errors gracefully - output basic status
        print("\033[31m[Claude] 📁 Error\033[0m")
        sys.exit(0)


if __name__ == '__main__':
    main()