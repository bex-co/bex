/* w8/m87 live check: libpq 17+ TLS cancel (PQcancelCreate/PQcancelBlocking)
 * against a pg-sni-proxy endpoint. Usage: cancel "<conninfo>" */
#include <libpq-fe.h>
#include <stdio.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
static double now(void) { struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t); return t.tv_sec + t.tv_nsec / 1e9; }
int main(int argc, char **argv) {
    PGconn *c = PQconnectdb(argv[1]);
    if (PQstatus(c) != CONNECTION_OK) { fprintf(stderr, "connect: %s", PQerrorMessage(c)); return 2; }
    PGresult *r = PQexec(c, "SELECT pg_backend_pid()");
    printf("libpq %d, backend pid %s, ssl %d\n", PQlibVersion(), PQgetvalue(r, 0, 0), PQsslInUse(c));
    char pid[32]; snprintf(pid, sizeof pid, "%s", PQgetvalue(r, 0, 0)); PQclear(r);
    double t0 = now();
    if (!PQsendQuery(c, "SELECT pg_sleep(12)")) { fprintf(stderr, "send: %s", PQerrorMessage(c)); return 2; }
    sleep(2);
    int ok;
    if (argc > 2 && strcmp(argv[2], "legacy") == 0) {
        char err[256] = "";
        PGcancel *pc = PQgetCancel(c);
        ok = PQcancel(pc, err, sizeof err);
        PQfreeCancel(pc);
        printf("legacy PQcancel ok=%d after %.2fs %s\n", ok, now() - t0, err);
    } else {
        PGcancelConn *cc = PQcancelCreate(c);
        ok = PQcancelBlocking(cc);
        printf("TLS PQcancelBlocking ok=%d after %.2fs %s\n", ok, now() - t0, ok ? "" : PQcancelErrorMessage(cc));
        PQcancelFinish(cc);
    }
    while ((r = PQgetResult(c)) != NULL) {
        const char *st = PQresultErrorField(r, PG_DIAG_SQLSTATE);
        printf("query ended after %.2fs: %s %s", now() - t0, PQresStatus(PQresultStatus(r)), st ? st : "");
        printf("\n"); PQclear(r);
    }
    r = PQexec(c, "SELECT pg_backend_pid()");
    printf("reuse: status %s, same pid %s\n", PQresStatus(PQresultStatus(r)), strcmp(PQgetvalue(r, 0, 0), pid) == 0 ? "yes" : "no");
    PQclear(r); PQfinish(c); return 0;
}
