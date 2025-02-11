# Display-MNDP

Display-MNDP is a Go-based tool designed to discover and display MikroTik devices using the MikroTik Neighbor Discovery Protocol (MNDP). This tool is cross-platform and works on both Windows and Linux.

## Features
- Discovers MikroTik devices on the network using MNDP
- Displays system name and IP addresses
- Runs as a service on Linux
- GUI version available for Windows

## Requirements
- Go 1.18 or later
- Windows 10/11 or Linux (tested on Ubuntu and Debian)
- MikroTik router with MNDP enabled

## Installation

### Windows
1. Download and extract the latest release from the [Releases](https://github.com/shtir/Display-MNDP/releases) page.
2. Open a terminal in the extracted folder.
3. Run the application:
   ```sh
   Display-MNDP.exe
   ```

### Linux
1. Clone the repository:
   ```sh
   git clone https://github.com/shtir/Display-MNDP.git
   cd Display-MNDP
   ```
2. Build the project:
   ```sh
   go build -o display-mndp
   ```
3. Run the application:
   ```sh
   ./display-mndp
   ```

## Usage
Run the program, and it will automatically detect MikroTik devices on the network and display their information.

## Configuration
To configure the tool for automatic startup on Linux, you can set it up as a systemd service:
```sh
sudo cp display-mndp /usr/local/bin/
sudo nano /etc/systemd/system/display-mndp.service
```
Add the following content:
```ini
[Unit]
Description=Display MikroTik MNDP Devices
After=network.target

[Service]
ExecStart=/usr/local/bin/display-mndp
Restart=always
User=root

[Install]
WantedBy=multi-user.target
```
Save and exit, then enable the service:
```sh
sudo systemctl enable display-mndp
sudo systemctl start display-mndp
```

## License
This project is licensed under the MIT License.

## Author
Developed by [shtir](https://github.com/shtir).

