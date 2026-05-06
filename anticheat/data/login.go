package data

// LoginData contains client-reported identity fields from the Login packet.
// These are used by login checks (Protocol/A, EditionFaker/A, ClientSpoof/A)
// to detect spoofed or cheat-client connections.
type LoginData struct {
	Protocol       int
	ClientVersion  string
	DeviceOS       uint32
	DeviceModel    string
	GameVersion    string
	ClientRandomID int64
}
