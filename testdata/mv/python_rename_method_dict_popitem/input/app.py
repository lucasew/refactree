class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_popitem(d: dict[str, A]):
    k, a = d.popitem()
    a.run()


def use_popitem_paren(d: dict[str, A]):
    (k, a) = d.popitem()
    a.run()


def use_popitem_b(d: dict[str, B]):
    k, b = d.popitem()
    b.run()


def use_popitem_assigned():
    d: dict[str, A] = {}
    k, a = d.popitem()
    a.run()
    e: dict[str, B] = {}
    k2, b = e.popitem()
    b.run()
