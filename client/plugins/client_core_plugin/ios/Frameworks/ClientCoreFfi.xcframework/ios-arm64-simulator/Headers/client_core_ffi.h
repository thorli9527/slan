#pragma once

#ifdef __cplusplus
extern "C" {
#endif

char *client_core_v2_version(void);
char *client_core_v2_default_state_json(void);
char *client_core_v2_initialize(const char *state_dir);
char *client_core_v2_service_request_json(const char *request_json);
void client_core_v2_free_string(char *value);

#ifdef __cplusplus
}
#endif
