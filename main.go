package main

import (
	"log"
	"net"

	"github.com/miekg/dns"
)

var db *databaseStruct
var forwardList string
var mapList string
var lg *logStruct

type handler struct{}

func (this *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg.Authoritative = true
		domain := msg.Question[0].Name
		res := db.table("map").
			where("domain", "=", strStrip(domain, ".")).
			first()
		if len(res) != 0 {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(toString(res["ip"])),
			})
		}
	}
	w.WriteMsg(&msg)
}

func main() {
	lg = getLogger()
	args := getArgparser("DNS 服务器")
	listenAddr := args.get("base", "listenAddr", "0.0.0.0", "监听的地址")
	listenPort := args.get("base", "listenPort", "53", "监听的端口")
	forwardList = args.get("base", "forwardList", "forwardList.txt", "只有允许的域名才会去上游dns查询记录并返回，井号之后是注释，如果井号之前是IP则跳过")
	mapList = args.get("base", "mapList", "mapList.txt", "域名和ip的键值对，逗号分割")
	checkInterval := args.getInt("base", "checkInterval", "60", "检查配置文件的间隔时间，秒")
	args.parseArgs()

	db = getSQLite(":memory:")
	db.createTable("map").
		addColumn("domain", "string").addIndex("domain").
		addColumn("ip", "string")

	go func() {
		m := db.table("map")
		for {
			data := open(forwardList).read() + " " + open(mapList).read()
			for _, r := range m.fields("domain").get() {
				if !strIn(toString(r["domain"]), data) {
					lg.trace("Delete", r["domain"])
					m.where("domain", "=", toString(r["domain"])).delete()
				}
			}

			for domain := range open(forwardList).readlines() {
				if len(strStrip(domain)) == 0 {
					continue
				}
				if strIn("#", domain) {
					domain = strStrip(strSplit(domain, "#")[0])
				} else {
					domain = strStrip(domain)
				}
				if len(reFindAll("[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+", domain)) != 0 {
					continue
				}
				ip := gethostbyname(domain)
				if m.where("domain", "=", domain).count() == 0 {
					lg.trace("Add", domain, ip)
					m.data(map[string]interface{}{
						"domain": domain,
						"ip":     ip,
					}).insert()
				} else {
					if m.where("domain", "=", domain).where("ip", "=", ip).count() == 0 {
						lg.trace("Update", domain, ip)
						m.where("domain", "=", domain).
							data(map[string]interface{}{
								"ip": ip,
							}).update()
					}
				}
			}

			for i := range open(mapList).readlines() {
				if len(strStrip(i)) == 0 {
					continue
				}
				//lg.debug("read from file:", i)
				domain := strSplit(strStrip(i), ",")[0]
				ip := strSplit(strStrip(i), ",")[1]
				if m.where("domain", "=", domain).count() == 0 {
					lg.trace("Add", domain, ip)
					m.data(map[string]interface{}{
						"domain": domain,
						"ip":     ip,
					}).insert()
				} else {
					if m.where("domain", "=", domain).where("ip", "=", ip).count() == 0 {
						lg.trace("Update", domain, ip)
						m.where("domain", "=", domain).
							data(map[string]interface{}{
								"ip": ip,
							}).update()
					}
				}
			}

			sleep(checkInterval)
		}
	}()

	srv := &dns.Server{Addr: listenAddr + ":" + listenPort, Net: "udp"}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to set udp listener %s\n", err.Error())
	}
}
