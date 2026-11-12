package io.nekohasekai.sagernet.fmt.v2ray

import com.google.gson.Gson
import com.google.gson.JsonObject
import com.google.gson.JsonParser
import java.util.UUID

/** Build only the local Xray hop. sing-box owns remote dialing and VPN protection. */
fun VMessBean.buildXhttpConfig(port: Int): String {
    require(isVLESS && type == "xhttp") { "XHTTP currently requires a VLESS node" }
    require(finalAddress == "127.0.0.1" && finalPort in 1..65535) {
        "XHTTP requires the protected local forwarding endpoint"
    }
    require(port in 1..65535) { "Invalid XHTTP local port" }
    require(!enableECH) { "ECH is not supported by this XHTTP integration" }
    require(!enableMux) { "Disable Multiplex for XHTTP nodes" }
    val protocols = alpn.split(',', '\n').map { it.trim() }.filter { it.isNotEmpty() }
    require(protocols.firstOrNull() != "h3") {
        "XHTTP HTTP/3 requires UDP forwarding and is not supported by this integration"
    }
    try {
        UUID.fromString(uuid)
    } catch (_: IllegalArgumentException) {
        throw IllegalArgumentException("Invalid VLESS user ID")
    }
    val mode = xhttpMode.ifBlank { "auto" }
    require(mode in setOf("auto", "packet-up", "stream-up", "stream-one")) {
        "Unsupported XHTTP mode"
    }
    val extra = if (xhttpExtra.isBlank()) JsonObject() else try {
        JsonParser.parseString(xhttpExtra).asJsonObject
    } catch (_: Exception) {
        throw IllegalArgumentException("XHTTP extra must be a JSON object")
    }
    require(!extra.has("downloadSettings")) {
        "Separate XHTTP downloadSettings are not supported by the protected forwarding integration"
    }
    val xhttp = mutableMapOf<String, Any>(
        "path" to path.ifBlank { "/" }, "mode" to mode, "extra" to extra,
    )
    xhttp["host"] = host.ifBlank {
        if (security == "tls" && sni.isNotBlank()) sni else serverAddress
    }
    val stream = mutableMapOf<String, Any>("network" to "xhttp", "xhttpSettings" to xhttp)
    val serverName = sni.ifBlank { serverAddress }
    if (security == "tls" && realityPubKey.isNotBlank()) {
        require(sni.isNotBlank()) { "REALITY requires a server name (SNI)" }
        stream["security"] = "reality"
        stream["realitySettings"] = mapOf(
            "serverName" to serverName,
            "fingerprint" to utlsFingerprint.ifBlank { "chrome" },
            "password" to realityPubKey,
            "shortId" to realityShortId,
        )
    } else if (security == "tls") {
        stream["security"] = "tls"
        val tls = mutableMapOf<String, Any>(
            "serverName" to serverName, "allowInsecure" to allowInsecure,
        )
        if (utlsFingerprint.isNotBlank()) tls["fingerprint"] = utlsFingerprint
        if (protocols.isNotEmpty()) tls["alpn"] = protocols
        if (certificates.isNotBlank()) tls["certificates"] = listOf(mapOf(
            "usage" to "verify", "certificate" to certificates.lines(),
        ))
        stream["tlsSettings"] = tls
    } else {
        require(security.isBlank() || security == "none") { "Unsupported XHTTP security mode" }
        stream["security"] = "none"
    }
    val user = mutableMapOf<String, Any>("id" to uuid, "encryption" to "none")
    if (encryption.isNotBlank() && encryption != "auto") user["flow"] = encryption
    return Gson().toJson(mapOf(
        "log" to mapOf("loglevel" to "warning"),
        "inbounds" to listOf(mapOf(
            "tag" to "local", "listen" to "127.0.0.1", "port" to port, "protocol" to "socks",
            "settings" to mapOf("auth" to "noauth", "udp" to true),
        )),
        "outbounds" to listOf(mapOf(
            "tag" to "xhttp", "protocol" to "vless",
            "settings" to mapOf("vnext" to listOf(mapOf(
                "address" to finalAddress, "port" to finalPort, "users" to listOf(user),
            ))),
            "streamSettings" to stream,
        )),
    ))
}
