class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_dict_values(d: dict[str, A]):
    for a in d.values():
        a.execute()


def use_dict_values_b(d: dict[str, B]):
    for b in d.values():
        b.run()


def use_dict_items(d: dict[str, A]):
    for k, a in d.items():
        a.execute()


def use_dict_items_b(d: dict[str, B]):
    for k, b in d.items():
        b.run()


def use_dict_values_comp(d: dict[str, A]):
    return [a.execute() for a in d.values()]


def use_dict_items_comp(d: dict[str, A]):
    return [a.execute() for k, a in d.items()]


def use_tuple_assign():
    a, b = A(), B()
    a.execute()
    b.run()


def use_tuple_assign_paren():
    (a, b) = A(), B()
    a.execute()
    b.run()


def use_for_tuple():
    for a, b in [(A(), B())]:
        a.execute()
        b.run()


def use_for_tuple_comp():
    return [a.execute() + b.run() for a, b in [(A(), B())]]
