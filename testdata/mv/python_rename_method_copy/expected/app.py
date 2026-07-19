class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_copy_index(items: list[A]):
    a = items.copy()[0]
    a.execute()


def use_copy_index_b(items: list[B]):
    b = items.copy()[0]
    b.run()


def use_copy_assign(items: list[A]):
    xs = items.copy()
    a = xs[0]
    a.execute()


def use_copy_assign_b(items: list[B]):
    ys = items.copy()
    b = ys[0]
    b.run()


def use_copy_for(items: list[A]):
    for a in items.copy():
        a.execute()


def use_copy_for_b(items: list[B]):
    for b in items.copy():
        b.run()


def use_or_empty(items: list[A]):
    for a in items or []:
        a.execute()


def use_or_empty_b(items: list[B]):
    for b in items or []:
        b.run()


def use_or_empty_index(items: list[A]):
    a = (items or [])[0]
    a.execute()


def use_or_empty_assign(items: list[A]):
    xs = items or []
    a = xs[0]
    a.execute()
    ys: list[B] = []
    zs = ys or []
    b = zs[0]
    b.run()


def use_or_literal(items: list[A]):
    a = (items or [A()])[0]
    a.execute()
    b = (items or [B()])[0]
    b.run()
