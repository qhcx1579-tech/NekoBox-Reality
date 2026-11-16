package io.nekohasekai.sagernet.ui

internal data class GroupPagerState<T>(
    val groups: List<T>,
    val selectedIndex: Int,
    val showTabs: Boolean,
)

/** Keep an empty default page when it is the only group; always label custom groups. */
internal fun <T> groupPagerState(
    groups: List<T>, selectedId: Long, emptyDefaultId: Long?,
    id: (T) -> Long, ungrouped: (T) -> Boolean,
): GroupPagerState<T> {
    val visible = groups.filter { groups.size == 1 || id(it) != emptyDefaultId }
    return GroupPagerState(
        visible,
        visible.indexOfFirst { id(it) == selectedId }.coerceAtLeast(0),
        visible.size > 1 || visible.any { !ungrouped(it) },
    )
}
