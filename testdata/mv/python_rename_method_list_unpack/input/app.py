class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_list_assign():
    [a, b] = [A(), B()]
    a.run()
    b.run()


def use_list_assign_expr():
    [a, b] = A(), B()
    a.run()
    b.run()


def use_list_from_tuple():
    [a, b] = (A(), B())
    a.run()
    b.run()
