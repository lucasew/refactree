class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_walrus_next(items: list[A]):
    if (a := next(iter(items))):
        a.run()


def use_walrus_next_b(items: list[B]):
    if (b := next(iter(items))):
        b.run()


def use_walrus_next_direct(items: list[A]):
    if (a := next(items)):
        a.run()


def use_walrus_min(items: list[A]):
    if (a := min(items)):
        a.run()


def use_walrus_max(items: list[A]):
    if (a := max(items)):
        a.run()


def use_walrus_min_key(items: list[A]):
    if (a := min(items, key=lambda x: 0)):
        a.run()


def use_walrus_pop(items: list[A]):
    if (a := items.pop()):
        a.run()


def use_walrus_pop_b(items: list[B]):
    if (b := items.pop()):
        b.run()


def use_walrus_sub(items: list[A]):
    if (a := items[0]):
        a.run()


def use_walrus_sub_b(items: list[B]):
    if (b := items[0]):
        b.run()


def use_walrus_assigned():
    xs = [A()]
    if (a := next(iter(xs))):
        a.run()
    ys = [B()]
    if (b := next(iter(ys))):
        b.run()


def use_walrus_reversed(items: list[A]):
    if (a := next(reversed(items))):
        a.run()


def use_walrus_filter(items: list[A]):
    if (a := next(filter(None, items))):
        a.run()
