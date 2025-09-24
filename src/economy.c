#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#define MAX_ITEMS 32
#define MAX_ITEM_NAME 48
#define MAX_SERVICE_NAME 48

#define MAX_TRANSACTION_HISTORY 64

static const double BANK_MIN_TRANSACTION = 1.0;
static const double BANK_MAX_TRANSACTION = 10000.0;
static const double BANK_MAX_DAILY_TOTAL = 25000.0;
static const size_t BANK_MAX_ITEM_QUANTITY = 999;

/*
 * A lightweight history entry that allows us to trace transactions that
 * occurred through the service NPC.  The history is intentionally small to
 * demonstrate how guardrails can cap resource usage while still providing a
 * helpful audit trail.
 */
typedef struct TransactionEntry {
    time_t timestamp;
    char description[96];
    double amount;
    double resulting_balance;
} TransactionEntry;

/*
 * The player's account combines currency and stored items.  Investments are
 * separated so the banking NPC can provide specialised services such as
 * locking balances for longer-term growth.
 */
typedef struct Account {
    char owner[64];
    double balance;
    double investment_balance;
    double daily_total;

    struct {
        char name[MAX_ITEM_NAME];
        size_t quantity;
    } items[MAX_ITEMS];
    size_t item_count;

    TransactionEntry history[MAX_TRANSACTION_HISTORY];
    size_t history_count;
} Account;

/*
 * The NPC service exposes its transactional guardrails.  The same code can be
 * re-used for different banking personalities by tweaking the limits and fees.
 */
typedef struct BankingService {
    char name[MAX_SERVICE_NAME];
    double deposit_fee;
    double withdrawal_fee;
    double transfer_fee;
    double investment_rate;
    double daily_limit;
} BankingService;

static void account_add_history(Account *account, const char *description, double amount)
{
    if (!account) {
        return;
    }

    TransactionEntry entry;
    entry.timestamp = time(NULL);
    snprintf(entry.description, sizeof(entry.description), "%s", description);
    entry.amount = amount;
    entry.resulting_balance = account->balance;

    if (account->history_count < MAX_TRANSACTION_HISTORY) {
        account->history[account->history_count++] = entry;
    } else {
        memmove(account->history, account->history + 1,
                sizeof(TransactionEntry) * (MAX_TRANSACTION_HISTORY - 1));
        account->history[MAX_TRANSACTION_HISTORY - 1] = entry;
    }
}

static void account_reset(Account *account, const char *owner, double initial_balance)
{
    if (!account) {
        return;
    }

    snprintf(account->owner, sizeof(account->owner), "%s", owner ? owner : "Unknown");
    account->balance = initial_balance < 0 ? 0 : initial_balance;
    account->investment_balance = 0;
    account->daily_total = 0.0;
    account->item_count = 0;
    account->history_count = 0;
}

static int enforce_amount(double amount, char *error, size_t error_size)
{
    if (amount < BANK_MIN_TRANSACTION) {
        if (error) {
            snprintf(error, error_size, "amount %.2f is below minimum transaction of %.2f",
                     amount, BANK_MIN_TRANSACTION);
        }
        return 0;
    }

    if (amount > BANK_MAX_TRANSACTION) {
        if (error) {
            snprintf(error, error_size, "amount %.2f exceeds maximum per transaction of %.2f",
                     amount, BANK_MAX_TRANSACTION);
        }
        return 0;
    }

    return 1;
}

static int enforce_daily(BankingService *service, Account *account, double amount,
                         char *error, size_t error_size)
{
    if (!service || !account) {
        return 0;
    }

    double limit = service->daily_limit > 0 ? service->daily_limit : BANK_MAX_DAILY_TOTAL;
    double projected = account->daily_total + amount;
    if (projected > limit) {
        if (error) {
            snprintf(error, error_size, "daily limit exceeded: %.2f / %.2f",
                     projected, limit);
        }
        return 0;
    }

    return 1;
}

static int apply_fee(BankingService *service, double *amount, double fee)
{
    if (!service || !amount) {
        return 0;
    }

    if (fee <= 0) {
        return 1;
    }

    double after_fee = *amount - fee;
    if (after_fee < BANK_MIN_TRANSACTION) {
        return 0;
    }
    *amount = after_fee;
    return 1;
}

int service_deposit(BankingService *service, Account *account, double amount,
                    char *error, size_t error_size)
{
    if (!service || !account) {
        return 0;
    }

    double requested = amount;

    if (!enforce_amount(amount, error, error_size) ||
        !enforce_daily(service, account, requested, error, error_size)) {
        return 0;
    }

    if (!apply_fee(service, &amount, service->deposit_fee)) {
        if (error) {
            snprintf(error, error_size, "deposit rejected due to service fee");
        }
        return 0;
    }

    account->balance += amount;
    account->daily_total += requested;
    account_add_history(account, "Deposit", amount);
    return 1;
}

int service_withdraw(BankingService *service, Account *account, double amount,
                     char *error, size_t error_size)
{
    if (!service || !account) {
        return 0;
    }

    double requested = amount;

    if (!enforce_amount(amount, error, error_size) ||
        !enforce_daily(service, account, requested, error, error_size)) {
        return 0;
    }

    if (!apply_fee(service, &amount, service->withdrawal_fee)) {
        if (error) {
            snprintf(error, error_size, "withdrawal rejected due to service fee");
        }
        return 0;
    }

    if (account->balance < amount) {
        if (error) {
            snprintf(error, error_size, "insufficient balance: have %.2f, need %.2f",
                     account->balance, amount);
        }
        return 0;
    }

    account->balance -= amount;
    account->daily_total += requested;
    account_add_history(account, "Withdrawal", -amount);
    return 1;
}

int service_transfer(BankingService *service, Account *from, Account *to, double amount,
                     char *error, size_t error_size)
{
    if (!service || !from || !to) {
        return 0;
    }

    if (!enforce_amount(amount, error, error_size) ||
        !enforce_daily(service, from, amount, error, error_size)) {
        return 0;
    }

    if (from->balance < amount) {
        if (error) {
            snprintf(error, error_size, "transfer failed: %.2f available, %.2f required",
                     from->balance, amount);
        }
        return 0;
    }

    double fee = service->transfer_fee;
    double credited = amount;
    if (fee > 0) {
        if (amount - fee < BANK_MIN_TRANSACTION) {
            if (error) {
                snprintf(error, error_size, "transfer failed: amount %.2f too small after fee",
                         amount);
            }
            return 0;
        }
        credited = amount - fee;
    }

    from->balance -= amount;
    to->balance += credited;
    from->daily_total += amount;

    account_add_history(from, "Transfer Sent", -amount);
    account_add_history(to, "Transfer Received", credited);
    return 1;
}

int service_invest(BankingService *service, Account *account, double amount,
                   char *error, size_t error_size)
{
    if (!service || !account) {
        return 0;
    }

    if (amount <= 0) {
        if (error) {
            snprintf(error, error_size, "investment requires positive amount");
        }
        return 0;
    }

    if (account->balance < amount) {
        if (error) {
            snprintf(error, error_size, "cannot invest %.2f with only %.2f balance",
                     amount, account->balance);
        }
        return 0;
    }

    account->balance -= amount;
    account->investment_balance += amount;
    account_add_history(account, "Investment Deposit", -amount);
    return 1;
}

void service_apply_investment_yield(BankingService *service, Account *account)
{
    if (!service || !account) {
        return;
    }

    if (account->investment_balance <= 0 || service->investment_rate <= 0) {
        return;
    }

    double yield = account->investment_balance * service->investment_rate;
    account->investment_balance += yield;
    account_add_history(account, "Investment Yield", yield);
}

int service_withdraw_investment(BankingService *service, Account *account, double amount,
                                char *error, size_t error_size)
{
    (void)service;

    if (!account) {
        return 0;
    }

    if (amount <= 0) {
        if (error) {
            snprintf(error, error_size, "withdraw amount must be positive");
        }
        return 0;
    }

    if (account->investment_balance < amount) {
        if (error) {
            snprintf(error, error_size, "only %.2f invested, cannot withdraw %.2f",
                     account->investment_balance, amount);
        }
        return 0;
    }

    account->investment_balance -= amount;
    account->balance += amount;
    account_add_history(account, "Investment Withdrawal", amount);
    return 1;
}

static size_t account_find_item(Account *account, const char *name)
{
    if (!account || !name) {
        return SIZE_MAX;
    }

    for (size_t i = 0; i < account->item_count; ++i) {
        if (strcmp(account->items[i].name, name) == 0) {
            return i;
        }
    }

    return SIZE_MAX;
}

int service_store_item(Account *account, const char *name, size_t quantity,
                       char *error, size_t error_size)
{
    if (!account || !name || quantity == 0) {
        if (error) {
            snprintf(error, error_size, "invalid item storage request");
        }
        return 0;
    }

    if (quantity > BANK_MAX_ITEM_QUANTITY) {
        if (error) {
            snprintf(error, error_size, "quantity %zu exceeds storage limit %zu",
                     quantity, BANK_MAX_ITEM_QUANTITY);
        }
        return 0;
    }

    size_t index = account_find_item(account, name);
    if (index == SIZE_MAX) {
        if (account->item_count >= MAX_ITEMS) {
            if (error) {
                snprintf(error, error_size, "storage vault is full");
            }
            return 0;
        }
        index = account->item_count++;
        snprintf(account->items[index].name, sizeof(account->items[index].name), "%s", name);
        account->items[index].quantity = 0;
    }

    if (account->items[index].quantity + quantity > BANK_MAX_ITEM_QUANTITY) {
        if (error) {
            snprintf(error, error_size, "total quantity would exceed limit %zu",
                     BANK_MAX_ITEM_QUANTITY);
        }
        return 0;
    }

    account->items[index].quantity += quantity;
    account_add_history(account, "Item Stored", (double)quantity);
    return 1;
}

int service_retrieve_item(Account *account, const char *name, size_t quantity,
                          char *error, size_t error_size)
{
    if (!account || !name || quantity == 0) {
        if (error) {
            snprintf(error, error_size, "invalid retrieval request");
        }
        return 0;
    }

    size_t index = account_find_item(account, name);
    if (index == SIZE_MAX) {
        if (error) {
            snprintf(error, error_size, "item '%s' not found in storage", name);
        }
        return 0;
    }

    if (account->items[index].quantity < quantity) {
        if (error) {
            snprintf(error, error_size, "only %zu of '%s' stored", account->items[index].quantity, name);
        }
        return 0;
    }

    account->items[index].quantity -= quantity;
    account_add_history(account, "Item Retrieved", -(double)quantity);

    if (account->items[index].quantity == 0) {
        for (size_t i = index; i + 1 < account->item_count; ++i) {
            account->items[i] = account->items[i + 1];
        }
        account->item_count--;
    }

    return 1;
}

void service_reset_daily(Account *account)
{
    if (!account) {
        return;
    }

    account->daily_total = 0.0;
}

void service_report(const BankingService *service, const Account *account)
{
    if (!service || !account) {
        return;
    }

    printf("\n=== %s Banking Report for %s ===\n", service->name, account->owner);
    printf("Balance: %.2f\n", account->balance);
    printf("Investments: %.2f\n", account->investment_balance);
    double limit = service->daily_limit > 0 ? service->daily_limit : BANK_MAX_DAILY_TOTAL;
    printf("Daily Total: %.2f / %.2f\n", account->daily_total, limit);

    printf("Stored Items (%zu):\n", account->item_count);
    for (size_t i = 0; i < account->item_count; ++i) {
        printf("  %s x%zu\n", account->items[i].name, account->items[i].quantity);
    }

    printf("Recent Transactions (%zu):\n", account->history_count);
    for (size_t i = 0; i < account->history_count; ++i) {
        char time_buf[32];
        struct tm *tm_info = localtime(&account->history[i].timestamp);
        strftime(time_buf, sizeof(time_buf), "%H:%M:%S", tm_info);
        printf("  [%s] %-20s %8.2f -> %.2f\n", time_buf,
               account->history[i].description,
               account->history[i].amount,
               account->history[i].resulting_balance);
    }
}

void service_init(BankingService *service, const char *name, double deposit_fee,
                  double withdrawal_fee, double transfer_fee, double investment_rate,
                  double daily_limit)
{
    if (!service) {
        return;
    }

    snprintf(service->name, sizeof(service->name), "%s", name ? name : "Bank");
    service->deposit_fee = deposit_fee;
    service->withdrawal_fee = withdrawal_fee;
    service->transfer_fee = transfer_fee;
    service->investment_rate = investment_rate;
    service->daily_limit = daily_limit <= 0 ? BANK_MAX_DAILY_TOTAL : daily_limit;
}

#ifdef ECONOMY_DEMO
int main(void)
{
    BankingService service;
    service_init(&service, "Guild Treasurer", 0.25, 0.5, 0.1, 0.05, 15000.0);

    Account hero;
    account_reset(&hero, "Hero", 500.0);

    char error[128];

    if (!service_deposit(&service, &hero, 200.0, error, sizeof(error))) {
        fprintf(stderr, "Deposit failed: %s\n", error);
    }

    if (!service_withdraw(&service, &hero, 50.0, error, sizeof(error))) {
        fprintf(stderr, "Withdraw failed: %s\n", error);
    }

    if (!service_store_item(&hero, "Ancient Relic", 2, error, sizeof(error))) {
        fprintf(stderr, "Storage failed: %s\n", error);
    }

    if (!service_invest(&service, &hero, 100.0, error, sizeof(error))) {
        fprintf(stderr, "Investment failed: %s\n", error);
    }

    service_apply_investment_yield(&service, &hero);

    service_report(&service, &hero);
    return 0;
}
#endif
