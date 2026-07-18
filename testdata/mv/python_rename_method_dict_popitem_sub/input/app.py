class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_popitem_sub(d: dict[str, A]):
    a = d.popitem()[1]
    a.run()


def use_popitem_sub_paren(d: dict[str, A]):
    a = (d.popitem())[1]
    a.run()


def use_popitem_sub_walrus(d: dict[str, A]):
    if (a := d.popitem()[1]):
        a.run()


def use_popitem_sub_b(d: dict[str, B]):
    b = d.popitem()[1]
    b.run()


def use_popitem_sub_assigned():
    d: dict[str, A] = {}
    a = d.popitem()[1]
    a.run()
    e: dict[str, B] = {}
    b = e.popitem()[1]
    b.run()
