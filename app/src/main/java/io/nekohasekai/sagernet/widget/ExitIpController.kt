package io.nekohasekai.sagernet.widget

import android.view.View
import android.widget.TextView
import androidx.lifecycle.lifecycleScope
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import io.nekohasekai.sagernet.R
import io.nekohasekai.sagernet.database.DataStore
import io.nekohasekai.sagernet.ui.MainActivity
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.util.Locale

/** Query results are held only in this view, never in preferences or logs. */
class ExitIpController(private val activity: MainActivity, private val view: TextView) {
    private var job: Job? = null
    private var generation = 0L

    init {
        view.setOnClickListener {
            if (!DataStore.serviceState.connected || job?.isActive == true) return@setOnClickListener
            MaterialAlertDialogBuilder(activity)
                .setTitle(R.string.exit_ip_title)
                .setMessage(R.string.exit_ip_privacy)
                .setPositiveButton(R.string.exit_ip_query) { _, _ -> query() }
                .setNegativeButton(android.R.string.cancel, null)
                .show()
        }
    }

    fun invalidate() {
        generation++
        job?.cancel()
        job = null
        view.isEnabled = true
        view.text = activity.getString(R.string.exit_ip_action)
        view.visibility = if (DataStore.serviceState.connected) View.VISIBLE else View.GONE
    }

    private fun query() {
        if (!DataStore.serviceState.connected || job?.isActive == true) return
        val expectedGeneration = generation
        val profile = DataStore.currentProfile
        val service = activity.connection.service ?: return
        fun current() = generation == expectedGeneration && DataStore.serviceState.connected &&
            DataStore.currentProfile == profile && activity.connection.service === service
        view.isEnabled = false
        view.text = activity.getString(R.string.exit_ip_loading)
        job = activity.lifecycleScope.launch {
            try {
                val result = withContext(Dispatchers.IO) { service.queryExitIP() }
                if (!current()) return@launch
                val parts = result.split('\n')
                check(parts.size == 2 && parts[0].length <= 45 && parts[1].matches(Regex("[A-Z]{2}")))
                val country = Locale("", parts[1]).getDisplayCountry(Locale.getDefault())
                    .ifBlank { parts[1] }
                view.text = activity.getString(R.string.exit_ip_result, parts[0], country)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                if (current()) view.text = activity.getString(when {
                    e.message.orEmpty().contains("exit_ip_reality_failed") -> R.string.exit_ip_reality_failed
                    e.message.orEmpty().contains("exit_ip_timeout") -> R.string.exit_ip_timeout
                    e.message.orEmpty().contains("exit_ip_invalid_response") -> R.string.exit_ip_invalid_response
                    else -> R.string.exit_ip_failed
                })
            } finally {
                if (generation == expectedGeneration) {
                    view.isEnabled = true
                    if (!current()) invalidate()
                }
            }
        }
    }
}
