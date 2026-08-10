package main

import "fmt"

func printUsage() {
	fmt.Printf("Usage: %sdrop%s send [--stay-open] [--mode p2p|mailbox] [--resume-passphrase <phrase>] <file> [file2...]\n", colorBold, colorReset)
	fmt.Printf("       %sdrop%s receive [--stay-open] [dir]\n", colorBold, colorReset)
	fmt.Printf("       %sdrop%s download [--stay-open] [--resume] <WORD WORD WORD NN> [dir]\n", colorBold, colorReset)
	fmt.Printf("       %sdrop%s discover [--stay-open] [dir]\n", colorBold, colorReset)
	fmt.Printf("       %sdrop%s update\n", colorBold, colorReset)
	fmt.Printf("       %sdrop%s version\n", colorBold, colorReset)
	fmt.Printf("       %sdrop%s help\n\n", colorBold, colorReset)
	fmt.Printf("Run %sdrop help%s for details.\n", colorBold, colorReset)
}

func printHelp() {
	b := colorBold
	r := colorReset
	d := colorDim
	fmt.Printf("\n%sdrop%s - peer-to-peer encrypted file transfer\n\n", b, r)

	fmt.Printf("%sCommands%s\n", b, r)
	fmt.Printf("  %ssend%s [--stay-open] [--mode p2p|mailbox] <file> [file2...]\n", b, r)
	fmt.Printf("      Encrypt and share a file. You get a passphrase like %sRIVER FALCON MARBLE 42%s.\n", b, r)
	fmt.Printf("      In %sp2p%s mode, multiple files stream in one session; a %sfolder%s is zipped.\n", b, r, b, r)
	fmt.Printf("      In %smailbox%s mode, multiple files ship in one upload without zipping;\n", b, r)
	fmt.Printf("        %sfolders%s (or mixes with folders) still become one zip upload.\n", b, r)
	fmt.Printf("        %s--mode p2p%s      Stream directly between sender and receiver over WebRTC.\n", b, r)
	fmt.Printf("        %s--mode mailbox%s  Upload to the relay; receiver downloads later.\n", b, r)
	fmt.Printf("                        10-minute expiry, 500 MB cap.\n")
	fmt.Printf("        %s--stay-open%s    After a p2p batch, prompt for more files (receiver needs %s--stay-open%s too).\n", b, r, b, r)
	fmt.Printf("        %s--resume-passphrase%s <phrase>  Reuse a passphrase instead of generating one, so a\n", b, r)
	fmt.Printf("                             retry after a dropped p2p transfer reopens the same room\n")
	fmt.Printf("                             (pair with %sdownload --resume%s on the other end).\n", b, r)
	fmt.Printf("        %s(default: prompt)%s\n\n", d, r)

	fmt.Printf("  %sreceive%s [--stay-open] [dir]\n", b, r)
	fmt.Printf("      Wait for someone to send a file to you. Defaults to current directory.\n")
	fmt.Printf("      %s--stay-open%s keeps the session open for more batches.\n\n", b, r)

	fmt.Printf("  %sdownload%s [--stay-open] [--resume] <WORD WORD WORD NN> [dir]\n", b, r)
	fmt.Printf("      Download mailbox or live payloads using a passphrase someone shared.\n")
	fmt.Printf("      Mailbox bundles with several files decrypt to separate saves.\n")
	fmt.Printf("      With live p2p, %s--stay-open%s waits for extra batches after the first.\n", b, r)
	fmt.Printf("      %s--resume%s picks up a single interrupted p2p file from its %s.part%s leftover\n", b, r, b, r)
	fmt.Printf("        in the destination dir (sender must retry with the same %s--resume-passphrase%s).\n\n", b, r)

	fmt.Printf("  %sdiscover%s [--stay-open] [dir]\n", b, r)
	fmt.Printf("      Look for %ssend%s sessions on the LAN (mDNS) instead of typing a passphrase blind.\n", b, r)
	fmt.Printf("      Pick a sender from the list, then still enter its passphrase to connect —\n")
	fmt.Printf("      discovery only narrows down %swho%s, it never transmits the passphrase itself.\n", b, r)
	fmt.Printf("      Falls back to suggesting %sdrop download%s if nothing is found.\n\n", b, r)

	fmt.Printf("  %supdate%s\n", b, r)
	fmt.Printf("      Self-update the CLI binary in place.\n\n")

	fmt.Printf("  %sversion%s\n", b, r)
	fmt.Printf("      Print the installed version.\n\n")

	fmt.Printf("  %shelp%s\n", b, r)
	fmt.Printf("      Show this message.\n\n")

	fmt.Printf("%sFlags%s\n", b, r)
	fmt.Printf("  %s--relay%s <origin>\n", b, r)
	fmt.Printf("      Use a relay other than the default for this call.\n")
	fmt.Printf("      Permanent: %sexport DROP_RELAY=https://your-relay.example%s\n\n", b, r)

	fmt.Printf("%sExamples%s\n", b, r)
	fmt.Printf("  drop send report.pdf\n")
	fmt.Printf("  drop send report.pdf notes.txt    %s# p2p: two files, one connection%s\n", d, r)
	fmt.Printf("  drop send photos/ docs/          %s# mailed or p2p with dirs: zip%s\n", d, r)
	fmt.Printf("  drop download RIVER FALCON MARBLE 42\n")
	fmt.Printf("  drop receive ~/Downloads\n")
	fmt.Printf("  drop --relay https://my.relay send a.zip\n\n")

	fmt.Printf("%sLinks%s\n", b, r)
	fmt.Printf("  Source        github.com/whaeuser/Airpipe\n")
	fmt.Printf("  Web sender    https://drop.example.com\n\n")
}
