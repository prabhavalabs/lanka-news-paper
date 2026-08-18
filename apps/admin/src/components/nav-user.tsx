import { EllipsisVerticalIcon, LogOutIcon, ShieldCheckIcon } from 'lucide-react'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/components/ui/sidebar'

type NavUserProps = {
  user: { email: string; name: string; role: string }
  onLogout: () => void | Promise<void>
}

export function NavUser({ user, onLogout }: NavUserProps) {
  const { isMobile } = useSidebar()
  const initials = user.name.split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase()

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<SidebarMenuButton size="lg" className="aria-expanded:bg-muted" />}
          >
            <Avatar className="size-8 rounded-lg">
              <AvatarFallback className="rounded-lg bg-primary text-primary-foreground">
                {initials}
              </AvatarFallback>
            </Avatar>
            <div className="grid flex-1 text-left text-sm leading-tight">
              <span className="truncate font-medium">{user.name}</span>
              <span className="truncate text-xs text-foreground/70">{user.email}</span>
            </div>
            <EllipsisVerticalIcon className="ml-auto size-4" />
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="min-w-60"
            side={isMobile ? 'bottom' : 'right'}
            align="end"
            sideOffset={4}
          >
            <DropdownMenuGroup>
              <DropdownMenuLabel className="font-normal">
                <div className="flex items-center gap-2">
                  <ShieldCheckIcon className="size-4 text-primary" />
                  <div>
                    <p className="font-medium">Administrator</p>
                    <p className="text-xs text-muted-foreground">Full newsroom access</p>
                  </div>
                </div>
              </DropdownMenuLabel>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={() => void onLogout()}>
              <LogOutIcon />
              Sign out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
