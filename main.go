package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	mndpPort      = 5678
	broadcastAddr = "255.255.255.255"
	webPort       = 5678
	version       = "0.0.2"
)

var (
	devices  = make(map[string]MNDPEntry)
	mutex    sync.Mutex
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients        = make(map[*websocket.Conn]bool)
	seqNo   uint16 = 0
)

type MNDPEntry struct {
	Timestamp    string
	Interface    string
	Board        string
	SourceMAC    string
	DeviceName   string
	Version      string
	IP           string
	IPv4_Address string
	Identity     string
	MAC          string
	Uptime       string
}

func main() {
	fmt.Println("Startup...")

	// *************** Log setting *************** //
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	log.SetOutput(file)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP(broadcastAddr),
		Port: mndpPort,
	})
	if err != nil {
		fmt.Println("Error creating UDP connection:", err)
		return
	}
	defer conn.Close()

	go func() {
		for {
			packet := createMNDPPacket()
			_, err := conn.Write(packet)
			if err != nil {
				fmt.Println("Error sending packet:", err)
				return
			}
			time.Sleep(10 * time.Second)
		}
	}()

	recvConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: mndpPort,
	})
	if err != nil {
		log.Fatal("Error creating UDP listener:", err)
	}
	defer recvConn.Close()

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, addr, err := recvConn.ReadFromUDP(buffer)
			if err != nil {
				log.Println("Error receiving packet:", err)
				continue
			}

			parsedData := parseMNDPPacket(buffer[:n])

			if parsedData["MAC"] != "" {
				mutex.Lock()
				devices[parsedData["MAC"]] = MNDPEntry{
					Timestamp:    time.Now().Local().Format("15:04:05"),
					Interface:    parsedData["Interface name"],
					SourceMAC:    parsedData["MAC"],
					DeviceName:   parsedData["Identity"],
					Version:      parsedData["Version"],
					IP:           addr.IP.String(),
					IPv4_Address: parsedData["IPv4-Address"],
					Uptime:       parsedData["Uptime"],
				}
				mutex.Unlock()
			}
			// log.Println("Devices map:", devices)

			// prettyJSON, err := json.MarshalIndent(response, "", "  ")
			// if err != nil {
			// 	fmt.Println("Error:", err)
			// 	return
			// }
			// fmt.Println(string(prettyJSON))

			broadcastUpdate()
		}
	}()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/ws", wsHandler)
	log.Printf("Web server started on http://0.0.0.0:%d\n", webPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", webPort), nil))
}

func broadcastUpdate() {

	// mutex.Lock()
	// defer mutex.Unlock()
	// data := make([]MNDPEntry, 0, len(devices))
	// for _, entry := range devices {
	// 	data = append(data, entry)
	// }
	for client := range clients {
		err := client.WriteJSON(devices)
		if err != nil {
			log.Println("WebSocket error:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	log.Println("New WebSocket client connected")
	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()
	broadcastUpdate()
	defer func() {
		mutex.Lock()
		delete(clients, conn)
		mutex.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket client disconnected")
			return
		}
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("index").Parse(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>MNDP Scanner</title>
		<style>
			body { font-family: Arial, sans-serif; background: #222; color: #fff; text-align: center; }
			table { width: 80%; margin: 20px auto; border-collapse: collapse; background: #333; }
			th, td { padding: 10px; border: 1px solid #555; }
			th { background: #444; }
		</style>
	</head>
	<body>
		<h2>Discovered MNDP Devices</h2>
		<table>
			<thead>
				<tr>
					<th>Time</th><th>Device Name</th><th>IP</th><th>IP v4</th><th>MAC</th><th>Uptime</th>
				</tr>
			</thead>
			<tbody id="deviceTable"></tbody>
		</table>
		<script>
			const socket = new WebSocket("ws://" + window.location.host + "/ws");
			socket.onmessage = function(event) {
			// console.log("Received:", event.data);
			    const devicesObj = JSON.parse(event.data);
			    const devices = Object.values(devicesObj);
				console.log("Parsed devices (after conversion):", devices);
			    const table = document.getElementById("deviceTable");
			    table.innerHTML = "";

			    devices.forEach(device => {
			        const row = document.createElement("tr");

			        const columns = [
			            device.Timestamp || "N/A",
						device.DeviceName || "N/A",
						device.IP || "N/A",
						device.IPv4_Address || "N/A",
			            device.SourceMAC || "N/A",
						device.Uptime || "N/A",
			            
			        ];

			        columns.forEach(value => {
			            const cell = document.createElement("td");
			            cell.textContent = value;  
			            row.appendChild(cell);
			        });

			        table.appendChild(row);
			    });
			};


		</script>
	</body>
	</html>`))
	tmpl.Execute(w, nil)
}

func createMNDPPacket() []byte {
	packet := []byte{0x00, 0x00}
	seqBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(seqBytes, seqNo)
	packet = append(packet, seqBytes...)
	seqNo++

	uptime := getUptimeSeconds()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "n/a"
	}
	ips := getIPAddresses()

	packet = append(packet, createTLV(1, "b8:27:eb:e0:dc:e8")...)
	packet = append(packet, createTLV(5, hostname)...)
	packet = append(packet, createTLV(7, version)...)
	packet = append(packet, createTLV(8, "MikroTik")...)
	packet = append(packet, createTLV(10, uptime)...)
	packet = append(packet, createTLV(11, "SHT1-SHT2")...)
	packet = append(packet, createTLV(12, "RPI-3")...)
	packet = append(packet, createTLV(14, "0")...)
	packet = append(packet, createTLV(17, ips)...)

	return packet
}

func createTLV(fieldType byte, value string) []byte {
	tlv := []byte{0x00, fieldType}
	var valueBytes []byte

	switch fieldType {
	case 1: // MAC Address
		mac := strings.ReplaceAll(value, ":", "")
		valueBytes, _ = hex.DecodeString(mac)

	case 10: // Uptime
		uptimeFloat, _ := strconv.ParseFloat(value, 64)
		uptime := uint32(uptimeFloat)
		uptimeBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(uptimeBytes, uptime)
		valueBytes = uptimeBytes

	case 17: // IPv4-Address
		ipParts := strings.Split(value, ".")
		for _, part := range ipParts {
			byteValue, _ := strconv.Atoi(part)
			valueBytes = append(valueBytes, byte(byteValue))
		}
	default:
		valueBytes = []byte(value)
	}

	length := uint16(len(valueBytes))
	lengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBytes, length)
	tlv = append(tlv, lengthBytes...)

	tlv = append(tlv, valueBytes...)

	return tlv
}

func parseMNDPPacket(packet []byte) map[string]string {
	result := make(map[string]string)
	offset := 4
	for offset < len(packet) {
		if offset+2 > len(packet) {
			fmt.Println("Low len")
			break
		}

		fieldType := packet[offset+1]
		fieldLength := int(uint16(packet[offset+2])<<8 | uint16(packet[offset+3]))
		// fmt.Printf("fieldType:%x \n fieldLength: %x \n\n", fieldType, fieldLength)
		offset += 4

		if offset+fieldLength > len(packet) {
			fmt.Println("End of Data OR Wrong data length")
			break
		}

		fieldValue := string(packet[offset : offset+fieldLength])
		offset += fieldLength

		switch fieldType {
		case 1:
			mac := strings.ToUpper(hex.EncodeToString([]byte(fieldValue)))
			macFormatted := strings.Join(splitEvery(mac, 2), ":")
			result["MAC"] = macFormatted
			// result["MAC"] = strings.ToUpper(hex.EncodeToString([]byte(fieldValue)))
		case 5:
			result["Identity"] = fieldValue
		case 7:
			result["Version"] = fieldValue
		case 8:
			result["Platform"] = fieldValue
		case 10:
			uptimeSeconds := binary.LittleEndian.Uint32(packet[offset-fieldLength : offset])
			duration := time.Duration(uptimeSeconds) * time.Second
			days := duration / (24 * time.Hour)
			hours := (duration % (24 * time.Hour)) / time.Hour
			minutes := (duration % time.Hour) / time.Minute
			seconds := (duration % time.Minute) / time.Second
			result["Uptime"] = fmt.Sprintf("%d days, %02d:%02d:%02d", days, hours, minutes, seconds)
		case 11:
			result["Software-ID"] = fieldValue
		case 12:
			result["Board"] = fieldValue
		case 14:
			result["Unpack"] = hex.EncodeToString([]byte(fieldValue))
		case 15:
			hexParts := make([]string, len(fieldValue)/2)
			for i := 0; i < len(fieldValue); i += 2 {
				hexParts[i/2] = fmt.Sprintf("%X%X", fieldValue[i], fieldValue[i+1])
			}
			result["IPv6-Address"] = strings.Join(hexParts, ":")
			// result["IPv6-Address"] = fieldValue
		case 16:
			result["Interface name"] = fieldValue
		case 17:
			strValues := make([]string, len(fieldValue))
			for i, v := range []byte(fieldValue) {
				strValues[i] = strconv.Itoa(int(v))
			}
			result["IPv4-Address"] = strings.Join(strValues, ".")
			// result["IPv4-Address"] = fmt.Sprintf("%d.%d.%d.%d", fieldValue[0], fieldValue[1], fieldValue[2], fieldValue[3])
		}

	}

	return result
}

func splitEvery(s string, n int) []string {
	var parts []string
	for i := 0; i < len(s); i += n {
		parts = append(parts, s[i:i+n])
	}
	return parts
}

func getUptimeSeconds() string {
	if uptimeData, err := os.ReadFile("/proc/uptime"); err == nil {
		uptimeParts := strings.Fields(string(uptimeData))
		return uptimeParts[0]
	}

	return strconv.FormatFloat(float64(time.Since(time.Now().Add(-time.Duration(time.Now().Unix())*time.Second)).Seconds()), 'f', 0, 64)
}

// func getIPAddresses() []string {
func getIPAddresses() string {
	// var ips []string
	var ips string
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Error getting network interfaces:", err)
		ips = "Error"
		return ips
	}

	for _, iface := range hostInterfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if v.IP.To4() != nil {
					// ips = append(ips, v.IP.String())
					ips = ips + v.IP.String() + "."
				}
			}
		}
	}
	return ips
}
