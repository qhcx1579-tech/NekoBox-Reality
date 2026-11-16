package io.nekohasekai.sagernet.ui

import org.junit.Assert.*
import org.junit.Test

class GroupPagerStateTest {
    private data class Group(val id: Long, val ungrouped: Boolean = false)
    private val default = Group(1, true)
    private val custom = Group(2)
    private fun state(groups: List<Group>, selected: Long = 1, emptyDefault: Long? = 1) =
        groupPagerState(groups, selected, emptyDefault, { it.id }, { it.ungrouped })

    @Test fun firstCustomGroupIsVisibleAfterReturningHome() {
        val page = state(listOf(default, custom))
        assertEquals(listOf(custom), page.groups)
        assertTrue(page.showTabs)
        assertEquals(0, page.selectedIndex)
    }

    @Test fun initialEmptyDefaultRetainsAnImportTarget() {
        val page = state(listOf(default))
        assertEquals(listOf(default), page.groups)
        assertFalse(page.showTabs)
        assertEquals(0, page.selectedIndex)
    }

    @Test fun populatedDefaultAndCustomAreBothSelectable() {
        val page = state(listOf(default, custom), 2, null)
        assertEquals(listOf(default, custom), page.groups)
        assertEquals(1, page.selectedIndex)
        assertTrue(page.showTabs)
    }

    @Test fun deletingSelectedGroupFallsBackToRemainingGroup() {
        val page = state(listOf(default, Group(3)), 2)
        assertEquals(listOf(Group(3)), page.groups)
        assertEquals(0, page.selectedIndex)
        assertTrue(page.showTabs)
    }

    @Test fun deletingLastCustomGroupRestoresDefaultPage() {
        val page = state(listOf(default), 2)
        assertEquals(listOf(default), page.groups)
        assertFalse(page.showTabs)
        assertEquals(0, page.selectedIndex)
    }

    @Test fun groupOrderAndCurrentSelectionSurviveReload() {
        val page = state(listOf(default, Group(4), custom, Group(3)), 2)
        assertEquals(listOf(Group(4), custom, Group(3)), page.groups)
        assertEquals(1, page.selectedIndex)
    }
}
