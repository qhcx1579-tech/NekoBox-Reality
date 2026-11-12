package io.nekohasekai.sagernet.bg

import kotlinx.coroutines.*
import org.junit.Assert.*
import org.junit.Test
import java.io.IOException
import java.net.InetAddress
import java.net.ServerSocket

class LocalSocksReadyTest {
    @Test fun waitsForHandshakeInsteadOfOnlyAnOpenPort() = runBlocking {
        ServerSocket(0, 1, InetAddress.getByName("127.0.0.1")).use { server ->
            val peer = async(Dispatchers.IO) {
                server.accept().use { socket ->
                    assertArrayEquals(byteArrayOf(5, 1, 0), socket.getInputStream().readNBytes(3))
                    delay(80)
                    socket.getOutputStream().write(byteArrayOf(5, 0))
                }
            }
            awaitLocalSocks(server.localPort, 2_000)
            peer.await()
        }
    }

    @Test fun closedPortTimesOut() = runBlocking {
        val port = ServerSocket(0).use { it.localPort }
        try {
            awaitLocalSocks(port, 200)
            fail("Expected startup timeout")
        } catch (_: IOException) { }
    }

    @Test fun wrongProtocolIsRejected() = runBlocking {
        ServerSocket(0, 1, InetAddress.getByName("127.0.0.1")).use { server ->
            val peer = launch(Dispatchers.IO) {
                server.accept().use { it.getOutputStream().write(byteArrayOf(4, 0)) }
            }
            try {
                awaitLocalSocks(server.localPort, 250)
                fail("Expected protocol rejection")
            } catch (_: IOException) { }
            peer.join()
        }
    }

    @Test fun startupWaitCanBeCancelled() = runBlocking {
        val port = ServerSocket(0).use { it.localPort }
        val wait = launch { awaitLocalSocks(port) }
        delay(50)
        withTimeout(1_000) { wait.cancelAndJoin() }
        assertTrue(wait.isCancelled)
    }
}
