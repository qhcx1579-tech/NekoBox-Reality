package io.nekohasekai.sagernet.fmt.v2ray

import com.google.gson.JsonParser
import io.nekohasekai.sagernet.database.ProxyEntity
import io.nekohasekai.sagernet.fmt.ConfigBuildResult
import org.junit.Assert.*
import org.junit.Test
import java.util.UUID

class XhttpExportTest {
    @Test fun exportContainsBothCoresWithMatchingLocalPorts() {
        val bean = VMessBean().apply {
            initializeDefaultValues()
            alterId = -1
            type = "xhttp"
            uuid = UUID.randomUUID().toString()
            serverAddress = "proxy.example"
            serverPort = 443
            finalAddress = "127.0.0.1"
            finalPort = 34567
        }
        val entity = ProxyEntity().putBean(bean)
        val singBox = "{\"outbounds\":[{\"type\":\"socks\",\"server_port\":23456}]}"
        val built = ConfigBuildResult(singBox,
            listOf(ConfigBuildResult.IndexEntity(linkedMapOf(23456 to entity))),
            0, emptyMap(), emptyMap(), -1)
        val (content, name) = entity.exportConfig(built)
        assertEquals("profiles.txt", name)
        val parts = content.split("\n\n")
        assertEquals(2, parts.size)
        assertEquals(singBox, parts[0])
        val xray = JsonParser.parseString(parts[1]).asJsonObject
        assertEquals(23456, xray.getAsJsonArray("inbounds")[0].asJsonObject["port"].asInt)
        val outbound = xray.getAsJsonArray("outbounds")[0].asJsonObject
        val endpoint = outbound.getAsJsonObject("settings").getAsJsonArray("vnext")[0].asJsonObject
        assertEquals(34567, endpoint["port"].asInt)
        assertEquals(bean.uuid, endpoint.getAsJsonArray("users")[0].asJsonObject["id"].asString)
    }
}
