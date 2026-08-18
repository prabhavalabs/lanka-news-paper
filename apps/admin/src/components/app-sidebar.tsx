import * as React from 'react'
import {
  LayoutDashboardIcon,
  ListChecksIcon,
  MessageSquareWarningIcon,
  NetworkIcon,
  NewspaperIcon,
  RadioTowerIcon,
  SparklesIcon,
} from 'lucide-react'
import { Link } from 'react-router'

import { NavMain, type NavItem } from '@/components/nav-main'
import { NavUser } from '@/components/nav-user'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'

const navigation: NavItem[] = [
  { title: 'Overview', url: '/', icon: LayoutDashboardIcon },
  { title: 'Sources', url: '/sources', icon: RadioTowerIcon },
  { title: 'Articles', url: '/articles', icon: NewspaperIcon },
  { title: 'Knowledge graph', url: '/knowledge', icon: NetworkIcon },
  { title: 'Editorial queue', url: '/queue', icon: ListChecksIcon },
  { title: 'Complaints', url: '/complaints', icon: MessageSquareWarningIcon },
  { title: 'AI & routing', url: '/routing', icon: SparklesIcon },
]

type AppSidebarProps = React.ComponentProps<typeof Sidebar> & {
  user: { email: string; name: string; role: string }
  onLogout: () => void | Promise<void>
}

export function AppSidebar({ user, onLogout, ...props }: AppSidebarProps) {
  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              render={<Link to="/" />}
              tooltip="News Control Room"
              className="data-[slot=sidebar-menu-button]:p-1.5!"
            >
              <img src="/brand/news-control-room-mark.svg" alt="" className="size-7" />
              <span className="text-base font-semibold tracking-tight">News Control Room</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={navigation} />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={user} onLogout={onLogout} />
      </SidebarFooter>
    </Sidebar>
  )
}
