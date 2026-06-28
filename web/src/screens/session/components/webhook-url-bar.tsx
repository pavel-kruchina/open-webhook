import { CodeHighlight } from '@mantine/code-highlight'
import { Text } from '@mantine/core'
import React from 'react'
import { useData } from '~/shared'

/**
 * A compact, always-visible bar showing the session's unique webhook URL. It is rendered above the request details
 * so the URL stays easy to find and copy even when a captured request is open.
 */
export const WebhookUrlBar: React.FC = () => {
  const { webHookUrl } = useData()

  if (!webHookUrl) {
    return null
  }

  return (
    <>
      <Text>Here&apos;s your unique URL:</Text>
      <CodeHighlight code={webHookUrl.toString()} language="bash" w="100%" mt="md" mb="lg" />
    </>
  )
}
