use relay_core::RelayTicket;

use crate::protocol::RelayTicketWire;

pub fn to_relay_ticket(ticket: RelayTicketWire) -> RelayTicket {
    RelayTicket {
        ticket_id: ticket.ticket_id,
        network_id: ticket.network_id,
        session_id: ticket.session_id,
        src_node_id: ticket.src_node_id,
        dst_node_id: ticket.dst_node_id,
        derp_cluster_id: ticket.derp_cluster_id,
        country_code: ticket.country_code,
        city_code: ticket.city_code,
        allowed_derp_node_ids: ticket.allowed_derp_node_ids,
        relay_url: ticket.relay_url,
        expires_at: ticket.expires_at,
        session_key: ticket.session_key,
        signature: ticket.signature,
    }
}
