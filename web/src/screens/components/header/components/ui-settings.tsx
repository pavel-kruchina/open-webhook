import type React from 'react'
import { ActionIcon, Button, Checkbox, Group, Stack, Text } from '@mantine/core'
import { IconMinus, IconPlus } from '@tabler/icons-react'
import { useBrowserNotifications, useSettings } from '~/shared'

const SCALE_MIN = 0.8
const SCALE_MAX = 1.4
const SCALE_STEP = 0.1

export const UISettings = (): React.JSX.Element => {
  const { autoNavigateToNewRequest, showRequestDetails, showNativeRequestNotifications, uiScale, updateSettings } =
    useSettings()
  const { granted, request } = useBrowserNotifications()

  const scale = uiScale ?? 1
  /** Clamp to the allowed range and round to avoid floating-point drift */
  const setScale = (value: number): void =>
    updateSettings({ uiScale: Math.round(Math.min(SCALE_MAX, Math.max(SCALE_MIN, value)) * 10) / 10 })

  /** Handle the change of the native notifications setting */
  const handleNativeNotificationsChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (granted) {
      updateSettings({ showNativeRequestNotifications: event.target.checked })
    } else {
      request().then((ok) => {
        updateSettings({ showNativeRequestNotifications: ok && !event.target.checked })
      })
    }
  }

  return (
    <Stack>
      <div>
        <Text size="sm" mb={4}>
          Interface scale
        </Text>
        <Group gap="xs">
          <ActionIcon
            variant="default"
            onClick={() => setScale(scale - SCALE_STEP)}
            disabled={scale <= SCALE_MIN}
            aria-label="Decrease interface scale"
          >
            <IconMinus size="1rem" />
          </ActionIcon>
          <Text size="sm" w={48} ta="center">
            {Math.round(scale * 100)}%
          </Text>
          <ActionIcon
            variant="default"
            onClick={() => setScale(scale + SCALE_STEP)}
            disabled={scale >= SCALE_MAX}
            aria-label="Increase interface scale"
          >
            <IconPlus size="1rem" />
          </ActionIcon>
          <Button variant="subtle" size="compact-xs" onClick={() => setScale(1)} disabled={scale === 1}>
            Reset
          </Button>
        </Group>
      </div>
      <Checkbox
        checked={autoNavigateToNewRequest}
        onChange={(event) => updateSettings({ autoNavigateToNewRequest: event.target.checked })}
        label="Automatically navigate to the new request"
      />
      <Checkbox
        checked={showRequestDetails}
        onChange={(event) => updateSettings({ showRequestDetails: event.target.checked })}
        label="Display request details"
      />
      <Checkbox
        checked={showNativeRequestNotifications}
        onChange={handleNativeNotificationsChange}
        label={
          <>
            <Text size="sm">Use native notifications for new requests (instead of the in-app ones)</Text>
            {!granted && (
              <Text size="sm" c="dimmed" fw={700}>
                Permission required
              </Text>
            )}
          </>
        }
      />
    </Stack>
  )
}
