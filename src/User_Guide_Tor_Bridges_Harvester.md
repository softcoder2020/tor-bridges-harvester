# Tor Bridges Harvester User Guide

**Tor Bridges Harvester** is a tool designed to scan Tor relays, identify reachable bridges, and generate configurations for use in Tor Browser, particularly in regions with internet censorship. It downloads relay data from the Tor Project's Onionoo service, tests relay connectivity, and outputs bridge configurations to a file. This guide covers how to compile, run, and use the program effectively.

## Table of Contents
1. [Overview](#overview)
2. [Requirements](#requirements)
3. [Installation and Compilation](#installation-and-compilation)
4. [Running the Program](#running-the-program)
5. [Command-Line Options](#command-line-options)
6. [Output Files](#output-files)
7. [Troubleshooting](#troubleshooting)
8. [Support](#support)

## Overview
Tor Bridges Harvester scans Tor relays to find reachable bridges, which are useful for bypassing network censorship. The program supports filtering relays by country or port, outputting results in Tor's `torrc` format, and integrating with Tor Browser's `prefs.js` configuration. It can also launch Tor Browser automatically after scanning.

There are two versions of the program:
- **Console Version** (`main_grok.go`): Displays progress and errors in the console.
- **Hidden Version** (`main_grok_hide_logs.go`): Runs without a console window, logging all output to `_scanner.log`.

Both versions append found bridges to `_bridges.txt` in real-time.

## Requirements
- **Operating System**: Windows (tested on Windows 10/11). Linux and macOS may work but are untested.
- **Go Compiler**: Go 1.18 or later (download from [golang.org](https://golang.org/dl/)).
- **Internet Connection**: Required to download relay data from Onionoo.
- **Tor Browser** (optional): Needed if using the `--browser` or `--start-browser` options.

## Installation and Compilation

1. **Install Go**:
   - Download and install Go from [golang.org](https://golang.org/dl/).
   - Verify installation by running `go version` in a terminal.

2. **Download the Source Code**:
   - Save the program source as `main_grok.go` (console version) or `main_grok_hide_logs.go` (hidden version).

3. **Compile the Program**:
   - Open a terminal in the directory containing the source file.
   - Compile the desired version:

     **Console Version**:
     ```bash
     go build -o tor-bridges-harvester.exe main_grok.go
     ```

     **Hidden Version** (no console window, logs to `_scanner.log`):
     ```bash
     go build -ldflags "-H=windowsgui" -o tor-bridges-harvester.exe main_grok_hide_logs.go
     ```

   - This creates an executable named `tor-bridges-harvester.exe`.

## Running the Program

1. **Console Version**:
   - Run the program from a terminal:
     ```bash
     .\tor-bridges-harvester.exe [options]
     ```
   - Progress and errors will appear in the terminal.

2. **Hidden Version**:
   - Double-click `tor-bridges-harvester.exe` or run it from a terminal:
     ```bash
     .\tor-bridges-harvester.exe [options]
     ```
   - No console window will appear. Check `_scanner.log` in the same directory for progress and errors.

3. **Default Behavior**:
   - Scans up to 5 reachable bridges.
   - Tests 30 relays concurrently.
   - Appends found bridges to `_bridges.txt` in real-time.
   - Outputs results to stdout (console version) or a specified file.

## Command-Line Options
The program supports the following options, passed as command-line arguments:

| Option | Description | Default |
|--------|-------------|---------|
| `-n <number>` | Number of relays to test concurrently | 30 |
| `-g <number>` | Target number of working bridges to find | 5 |
| `-c <countries>` | Comma-separated list of preferred/excluded/exclusive countries (e.g., `US,!CA,-RU`). Use `!` for exclusive, `-` for excluded | "" |
| `-timeout <seconds>` | Socket connection timeout in seconds | 10.0 |
| `-o <file>` | Output file for bridge configurations | stdout |
| `-torrc` | Output in `torrc` format (adds `Bridge` prefix and `UseBridges 1`) | false |
| `-proxy <url>` | Proxy for downloading relay data (e.g., `http://proxy:port`) | "" |
| `-url <urls>` | Comma-separated list of alternative Onionoo URLs | (default Onionoo URLs) |
| `-p <ports>` | Comma-separated list of ports to filter (e.g., `443,9001`) | "" |
| `-browser <path>` | Path to Tor Browser's `prefs.js` file for configuration | "" |
| `-start-browser` | Launch Tor Browser after scanning | false |

**Example**:
```bash
.\tor-bridges-harvester.exe -n 50 -g 10 -c "US,GB" -p 443 -o bridges.txt -torrc
```
This tests 50 relays at a time, aims for 10 bridges, filters for US/GB relays on port 443, and outputs to `bridges.txt` in `torrc` format.

## Output Files
- **_bridges.txt**:
  - Contains all found bridges in the format `<address> <fingerprint>`.
  - Updated in real-time as bridges are discovered.
  - Appends new bridges without overwriting existing content.

- **_scanner.log** (Hidden Version only):
  - Logs all progress, errors, and results.
  - Located in the same directory as the executable.
  - Appends new logs without overwriting.

- **Output File** (if specified with `-o`):
  - Contains the final bridge configurations, optionally in `torrc` format.
  - Overwrites the file if it exists.

## Troubleshooting
- **"Failed to download relay data"**:
  - Check your internet connection.
  - Use the `-proxy` option if Onionoo is blocked.
  - Specify alternative URLs with `-url`.

- **No bridges found**:
  - Increase the timeout with `-timeout` (e.g., `-timeout 20`).
  - Test more relays with `-n` (e.g., `-n 100`).
  - Remove country/port filters (omit `-c` or `-p`).

- **Hidden Version: No output visible**:
  - Check `_scanner.log` for progress and errors.
  - Ensure `_bridges.txt` is being updated.

- **Tor Browser not launching**:
  - Verify the Tor Browser path is correct.
  - Ensure `Browser/start-tor-browser` or `Browser/firefox.exe` exists.

- **IPv6 connection errors**:
  - Some IPv6 addresses may be unreachable due to network limitations.
  - Consider filtering for specific ports (e.g., `-p 443`) to focus on common configurations.

## Support
For issues, suggestions, or contributions:
- Visit the project repository (if available).
- Contact the developer via email or issue tracker (details TBD).
- Check `_scanner.log` for detailed error messages when reporting issues.

---

**Note**: This program is intended for use in regions with internet censorship. Ensure compliance with local laws and Tor Project guidelines when using bridges.