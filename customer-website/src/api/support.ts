import { apiClient } from './client'
import type { SupportTicket, SupportMessage, CreateTicketRequest, ReplyRequest } from '../types'

export const createTicket = (data: CreateTicketRequest) =>
  apiClient
    .post<{ ticket: SupportTicket; message: SupportMessage }>('/support/tickets', data)
    .then((r) => r.data)

export const listMyTickets = () =>
  apiClient.get<SupportTicket[]>('/support/tickets').then((r) => r.data)

export const getTicketMessages = (id: number) =>
  apiClient
    .get<{ ticket: SupportTicket; messages: SupportMessage[] }>(`/support/tickets/${id}/messages`)
    .then((r) => r.data)

export const replyToTicket = (id: number, data: ReplyRequest) =>
  apiClient.post<SupportMessage>(`/support/tickets/${id}/messages`, data).then((r) => r.data)