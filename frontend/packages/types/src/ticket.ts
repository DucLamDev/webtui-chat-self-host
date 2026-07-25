import type { Id, ISODateTime } from "./api";

export type TicketStatus = "open" | "pending" | "resolved" | "closed";
export type TicketPriority = "low" | "normal" | "high" | "urgent";

export type Ticket = {
  id: Id;
  workspace_id: Id;
  channel_id?: Id | null;
  title: string;
  description: string;
  status: TicketStatus;
  priority: TicketPriority;
  created_by?: Id | null;
  assigned_to?: Id | null;
  resolved_at?: ISODateTime | null;
  closed_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type CreateTicketInput = {
  channel_id?: Id;
  title: string;
  description?: string;
  priority?: TicketPriority;
  assigned_to?: Id;
};

export type UpdateTicketInput = {
  channel_id?: Id | "";
  title?: string;
  description?: string;
  status?: TicketStatus;
  priority?: TicketPriority;
  assigned_to?: Id | "";
};
