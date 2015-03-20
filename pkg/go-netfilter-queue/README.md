go-netfilter-queue
==================

Go bindings for libnetfilter_queue

This library provides access to packets in the IPTables netfilter queue (NFQUEUE).
The libnetfilter_queue library is part of the [Netfilter project| http://netfilter.org/projects/libnetfilter_queue/].

Example
=======

use IPTables to direct all outgoing Ping/ICMP requests to the queue 0:

    iptables -A OUTPUT -p icmp -j NFQUEUE --queue-num 0

You can then use go-netfilter-queue to inspect the packets:

    package main
    
    import (
            "github.com/openshift/geard/pkg/go-netfilter-queue"
            "log"
    )
    
    func main() {
            nfq, err := netfilter.NewNFQueue(0, 100, netfilter.NF_DEFAULT_PACKET_SIZE)
            if err != nil {
                    log.Fatal(err)
            }
            defer nfq.Close()
            packets := nfq.GetPackets()
    
            for {
                    select {
                    case p := <-packets:
                            log.Println(p.Packet)
                            p.SetVerdict(netfilter.NF_ACCEPT)
                    }
            }
    }

To undo the IPTables redirect. Run:

    iptables -D OUTPUT -p icmp -j NFQUEUE --queue-num 0
