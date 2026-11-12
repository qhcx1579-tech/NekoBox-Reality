package io.nekohasekai.sagernet.bg

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import java.io.IOException
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.TimeUnit

/** A SOCKS handshake verifies readiness without making any remote connection. */
internal suspend fun awaitLocalSocks(port: Int, timeoutMillis: Long = 10_000) {
    require(port in 1..65535 && timeoutMillis > 0)
    val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis)
    while (true) {
        currentCoroutineContext().ensureActive()
        val remaining = TimeUnit.NANOSECONDS.toMillis(deadline - System.nanoTime())
        if (remaining <= 0) throw IOException("XHTTP local SOCKS service did not become ready in time")
        val ready = withContext(Dispatchers.IO) {
            try {
                Socket().use { socket ->
                    val attemptTimeout = remaining.coerceAtMost(150).coerceAtLeast(1).toInt()
                    socket.connect(InetSocketAddress("127.0.0.1", port), attemptTimeout)
                    socket.soTimeout = attemptTimeout
                    socket.getOutputStream().apply {
                        write(byteArrayOf(5, 1, 0))
                        flush()
                    }
                    val input = socket.getInputStream()
                    input.read() == 5 && input.read() == 0
                }
            } catch (_: IOException) {
                false
            }
        }
        if (ready) return
        delay(50)
    }
}
