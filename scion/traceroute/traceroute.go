// Copyright 2020 Anapaya Systems
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package traceroute implements tracerouting based on SCMP traceroute messages.
package traceroute

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/log"
	"github.com/scionproto/scion/pkg/private/common"
	"github.com/scionproto/scion/pkg/private/serrors"
	"github.com/scionproto/scion/pkg/slayers/path/scion"
	"github.com/scionproto/scion/pkg/snet"
	"github.com/scionproto/scion/pkg/snet/path"
)

// Update contains the information for a single hop.
type Update struct {
	// Index indicates the hop index in the path.
	Index int
	// Remote is the remote router.
	Remote snet.SCIONAddress
	// Interface is the interface ID of the remote router.
	Interface uint64
	// RTTs are the RTTs for this hop. To detect whether there was a timeout the
	// value of the RTT can be compared against the timeout value from the
	// configuration.
	RTTs []time.Duration
}

func (u Update) empty() bool {
	return u.Index == 0 && u.Remote == (snet.SCIONAddress{}) && u.Interface == 0 && len(u.RTTs) == 0
}

// Stats contains the amount of sent and received packets.
type Stats struct {
	Sent, Recv uint
}

// Config configures the traceroute run.
type Config struct {
	Local   addr.Addr
	Remote  addr.Addr
	NextHop *net.UDPAddr

	Topology    snet.Topology
	MTU         uint16
	PathEntry   snet.Path
	PayloadSize uint
	Timeout     time.Duration
	EPIC        bool

	// ProbesPerHop indicates how many probes should be done per hop.
	ProbesPerHop int
	// ErrHandler is invoked for every error that does not cause tracerouting to
	// abort. Execution time must be small, as it is run synchronously.
	ErrHandler func(error)
	// Update handler is invoked for every hop. Execution time must be
	// small, as it is run synchronously.
	UpdateHandler func(Update)
}

type tracerouter struct {
	probesPerHop  int
	timeout       time.Duration
	conn          snet.PacketConn
	local         addr.Addr
	remote        addr.Addr
	errHandler    func(error)
	updateHandler func(Update)

	replies <-chan reply

	path    snet.Path
	nextHop *net.UDPAddr
	epic    bool
	id      uint16
	index   int

	stats Stats
}

// Run runs the traceroute.
func Run(ctx context.Context, cfg Config) (Stats, error) {
	if _, isEmpty := cfg.PathEntry.Dataplane().(path.Empty); isEmpty {
		return Stats{}, serrors.New("empty path is not allowed for traceroute")
	}
	replies := make(chan reply, 10)
	sn := &snet.SCIONNetwork{
		SCMPHandler: scmpHandler{replies: replies},
		Topology:    cfg.Topology,
	}

	// We need to manufacture a netip.UDPAddr as we're constrained by the sn API.
	netUdpAddr := net.UDPAddrFromAddrPort(netip.AddrPortFrom(cfg.Local.Host.IP(), 0))
	conn, err := sn.OpenRaw(ctx, netUdpAddr)
	if err != nil {
		return Stats{}, err
	}
	// Get our real local address.
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	asNetipAddr, ok := netip.AddrFromSlice(localAddr.IP)
	if !ok {
		panic("invalid local IP address: " + localAddr.IP.String())
	}
	local := cfg.Local
	local.Host = addr.HostIP(asNetipAddr)
	t := tracerouter{
		probesPerHop:  cfg.ProbesPerHop,
		timeout:       cfg.Timeout,
		conn:          conn,
		local:         local,
		remote:        cfg.Remote,
		replies:       replies,
		errHandler:    cfg.ErrHandler,
		updateHandler: cfg.UpdateHandler,
		id:            uint16(localAddr.Port),
		path:          cfg.PathEntry,
		nextHop:       cfg.NextHop,
		epic:          cfg.EPIC,
	}
	return t.Traceroute(ctx)
}

func (t *tracerouter) Traceroute(ctx context.Context) (Stats, error) {
	scionPath, ok := t.path.Dataplane().(path.SCION)
	if !ok {
		return Stats{}, serrors.New("only SCION path allowed for traceroute",
			"type", common.TypeOf(t.path.Dataplane()))
	}

	var idxPath scion.Decoded
	if err := idxPath.DecodeFromBytes(scionPath.Raw); err != nil {
		return t.stats, serrors.Wrap("decoding path", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer log.HandlePanic()
		t.drain(ctx)
	}()
	prevXover := false
	for i := 0; i < len(idxPath.HopFields); i++ {
		hf := idxPath.PathMeta.CurrHF
		info := idxPath.InfoFields[idxPath.PathMeta.CurrINF]
		// First hop of the path isn't probed, since only the egress hop is
		// relevant.
		// After a crossover (segment change) only the egress interface is
		// relevant, since the ingress interface is in previous hop.
		if i != 0 && !prevXover {
			u, err := t.probeHop(ctx, hf, !info.ConsDir)
			if err != nil {
				return t.stats, serrors.Wrap("probing hop", err, "hop_index", i)
			}
			if t.updateHandler != nil && !u.empty() {
				t.updateHandler(u)
			}
		}
		// Peering links do not count as regular cross
		// overs. For peering links we probe all interfaces on
		// the path.
		xover := idxPath.IsXover() && !info.Peer
		// The last hop of the path isn't probed, only the ingress interface is
		// relevant.
		// At a crossover (segment change) only the ingress interface is
		// relevant, since the egress interface is in the next hop.
		if i < len(idxPath.HopFields)-1 && !xover {
			u, err := t.probeHop(ctx, hf, info.ConsDir)
			if err != nil {
				return t.stats, serrors.Wrap("probing hop", err, "hop_index", i)
			}
			if t.updateHandler != nil && !u.empty() {
				t.updateHandler(u)
			}
		}
		if i < len(idxPath.HopFields)-1 {
			if err := idxPath.IncPath(); err != nil {
				return t.stats, serrors.Wrap("incrementing path", err)
			}
		}
		prevXover = xover
	}
	return t.stats, nil
}

func (t *tracerouter) probeHop(ctx context.Context, hfIdx uint8, egress bool) (Update, error) {
	var decoded scion.Decoded
	if err := decoded.DecodeFromBytes(t.path.Dataplane().(path.SCION).Raw); err != nil {
		return Update{}, serrors.Wrap("decoding path", err)
	}

	hf := &decoded.HopFields[hfIdx]
	if egress {
		hf.EgressRouterAlert = true
	} else {
		hf.IngressRouterAlert = true
	}

	scionAlertPath, err := path.NewSCIONFromDecoded(decoded)
	if err != nil {
		return Update{}, serrors.Wrap("setting alert flag", err)
	}

	var alertPath snet.DataplanePath
	if t.epic {
		epicAlertPath, err := path.NewEPICDataplanePath(
			scionAlertPath,
			t.path.Metadata().EpicAuths,
		)
		if err != nil {
			return Update{}, err
		}
		alertPath = epicAlertPath
	} else {
		alertPath = scionAlertPath
	}

	u := Update{
		Index: t.index,
		RTTs:  make([]time.Duration, 0, t.probesPerHop),
	}
	t.index++

	pkt := &snet.Packet{
		PacketInfo: snet.PacketInfo{
			Destination: t.remote,
			Source:      t.local,
			Path:        alertPath,
			Payload:     snet.SCMPTracerouteRequest{Identifier: t.id},
		},
	}
	for i := 0; i < t.probesPerHop; i++ {
		sendTs := time.Now()
		t.stats.Sent++
		if err := t.conn.WriteTo(pkt, t.nextHop); err != nil {
			return u, serrors.Wrap("writing", err)
		}
		select {
		case <-time.After(t.timeout):
			u.RTTs = append(u.RTTs, t.timeout+1)
			continue
		case reply := <-t.replies:
			if reply.Error != nil {
				if t.errHandler != nil {
					t.errHandler(reply.Error)
				}
				continue
			}
			if t.id != reply.Reply.Identifier {
				if t.errHandler != nil {
					t.errHandler(serrors.New("wrong SCMP ID",
						"expected", t.id, "actual", reply.Reply.Identifier))
				}
				continue
			}
			t.stats.Recv++
			rtt := reply.Received.Sub(sendTs).Round(time.Microsecond)
			u.RTTs = append(u.RTTs, rtt)
			u.Interface = reply.Reply.Interface
			u.Remote = reply.Remote
		case <-ctx.Done():
			return u, nil
		}
	}
	return u, nil
}

func (t tracerouter) drain(ctx context.Context) {
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var pkt snet.Packet
			var ov net.UDPAddr
			if err := t.conn.ReadFrom(&pkt, &ov); err != nil && t.errHandler != nil {
				// Rate limit the error reports.
				if now := time.Now(); now.Sub(last) > 500*time.Millisecond {
					t.errHandler(serrors.Wrap("reading packet", err))
					last = now
				}
			}
		}
	}
}

type reply struct {
	Received time.Time
	Reply    snet.SCMPTracerouteReply
	Remote   snet.SCIONAddress
	Error    error
}

type scmpHandler struct {
	replies chan<- reply
}

func (h scmpHandler) Handle(pkt *snet.Packet) error {
	r, err := h.handle(pkt)

	h.replies <- reply{
		Received: time.Now(),
		Reply:    r,
		Remote:   pkt.Source,
		Error:    err,
	}
	return nil
}

func (h scmpHandler) handle(pkt *snet.Packet) (snet.SCMPTracerouteReply, error) {
	if pkt.Payload == nil {
		return snet.SCMPTracerouteReply{}, serrors.New("no payload found")
	}
	r, ok := pkt.Payload.(snet.SCMPTracerouteReply)
	if !ok {
		return snet.SCMPTracerouteReply{}, serrors.New("not SCMPTracerouteReply",
			"type", common.TypeOf(pkt.Payload))
	}
	return r, nil
}
