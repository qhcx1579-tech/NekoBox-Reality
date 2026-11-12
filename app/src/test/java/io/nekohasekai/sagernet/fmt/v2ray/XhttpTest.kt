package io.nekohasekai.sagernet.fmt.v2ray

import com.google.gson.JsonParser
import com.esotericsoftware.kryo.io.ByteBufferOutput
import io.nekohasekai.sagernet.fmt.KryoConverters
import org.junit.Assert.*
import org.junit.Test
import java.io.File
import java.util.UUID
import java.util.Base64
import java.security.SecureRandom

class XhttpTest {
    private fun node() = VMessBean().apply {
        initializeDefaultValues()
        alterId = -1
        uuid = UUID.randomUUID().toString()
        serverAddress = "proxy.example"
        serverPort = 443
        type = "xhttp"
        host = "cdn.example"
        path = "/transport/"
        security = "tls"
        sni = "tls.example"
        xhttpMode = "packet-up"
        xhttpExtra = "{\"noSSEHeader\":true}"
    }

    @Test fun shareLinkAndStorageKeepXhttpParameters() {
        val original = node()
        val imported = parseV2Ray(original.toUriVMessVLESSTrojan(false)) as VMessBean
        imported.initializeDefaultValues()
        val restored = KryoConverters.deserialize(VMessBean(), KryoConverters.serialize(imported))
        assertTrue(restored.isVLESS)
        assertEquals("xhttp", restored.type)
        assertEquals(original.host, restored.host)
        assertEquals(original.path, restored.path)
        assertEquals(original.xhttpMode, restored.xhttpMode)
        assertEquals(original.xhttpExtra, restored.xhttpExtra)
    }

    @Test fun tcpStorageStillKeepsTlsAndFlow() {
        val original = node().apply { type = "tcp"; encryption = "xtls-rprx-vision" }
        val restored = original.clone()
        assertEquals("tcp", restored.type)
        assertEquals("tls", restored.security)
        assertEquals(original.encryption, restored.encryption)
        assertEquals(original.sni, restored.sni)
        assertEquals(original.uuid, restored.uuid)
    }

    @Test fun legacyVersion4TcpProfileRemainsReadable() {
        val original = node().apply {
            type = "tcp"; encryption = "xtls-rprx-vision"
            name = "Legacy profile"
            customOutboundJson = "{}"
        }
        val body = ByteBufferOutput(1024, -1).also { original.serialize(it) }.toBytes()
        val newFields = ByteBufferOutput(128, -1).apply {
            writeString(original.xhttpMode); writeString(original.xhttpExtra)
        }.toBytes()
        val current = KryoConverters.serialize(original)
        val legacy = current.copyOfRange(0, body.size - newFields.size) + current.copyOfRange(body.size, current.size)
        val header = ByteBufferOutput(4, -1).apply { writeInt(4) }.toBytes()
        header.copyInto(legacy)
        val restored = KryoConverters.deserialize(VMessBean(), legacy)
        assertEquals("tcp", restored.type)
        assertEquals(original.name, restored.name)
        assertEquals(original.uuid, restored.uuid)
        assertEquals(original.sni, restored.sni)
        assertEquals(original.encryption, restored.encryption)
        assertEquals("auto", restored.xhttpMode)
        assertEquals("", restored.xhttpExtra)
    }

    @Test fun generatedConfigUsesProtectedHopAndOriginalTlsName() {
        val bean = node().apply { finalAddress = "127.0.0.1"; finalPort = 34567 }
        val config = bean.buildXhttpConfig(23456)
        val json = JsonParser.parseString(config).asJsonObject
        val outbound = json.getAsJsonArray("outbounds")[0].asJsonObject
        val endpoint = outbound.getAsJsonObject("settings").getAsJsonArray("vnext")[0].asJsonObject
        assertEquals("127.0.0.1", endpoint["address"].asString)
        assertEquals(34567, endpoint["port"].asInt)
        val stream = outbound.getAsJsonObject("streamSettings")
        assertEquals("xhttp", stream["network"].asString)
        assertEquals("tls.example", stream.getAsJsonObject("tlsSettings")["serverName"].asString)
        assertEquals("cdn.example", stream.getAsJsonObject("xhttpSettings")["host"].asString)
        System.getenv("XHTTP_TEST_CONFIG_DIR")?.let {
            File(it).mkdirs()
            File(it, "generated-tls.json").writeText(config)
        }
    }

    @Test fun independentDownloadHopIsRejected() {
        val bean = node().apply {
            finalAddress = "127.0.0.1"; finalPort = 34567
            xhttpExtra = "{\"downloadSettings\":{\"address\":\"another.example\"}}"
        }
        assertThrows(IllegalArgumentException::class.java) { bean.buildXhttpConfig(23456) }
    }

    @Test fun realityConfigKeepsAuthenticationParameters() {
        val bean = node().apply {
            finalAddress = "127.0.0.1"; finalPort = 34567
            realityPubKey = Base64.getUrlEncoder().withoutPadding().encodeToString(ByteArray(32).also { SecureRandom().nextBytes(it) })
            realityShortId = "1234"
            utlsFingerprint = "chrome"
        }
        val config = bean.buildXhttpConfig(23456)
        val outbound = JsonParser.parseString(config).asJsonObject.getAsJsonArray("outbounds")[0].asJsonObject
        val stream = outbound.getAsJsonObject("streamSettings")
        assertEquals("reality", stream["security"].asString)
        assertEquals(bean.realityPubKey, stream.getAsJsonObject("realitySettings")["password"].asString)
        assertEquals(bean.realityShortId, stream.getAsJsonObject("realitySettings")["shortId"].asString)
        System.getenv("XHTTP_TEST_CONFIG_DIR")?.let { File(it, "generated-reality.json").writeText(config) }
    }

    @Test fun http3IsRejectedInsteadOfUsingAnUnprotectedRemoteSocket() {
        val bean = node().apply {
            finalAddress = "127.0.0.1"; finalPort = 34567; alpn = ", h3"
        }
        assertThrows(IllegalArgumentException::class.java) { bean.buildXhttpConfig(23456) }
    }
}
