/*
    Copyright (c) 2007-2016 Contributors as noted in the AUTHORS file

    This file is part of libzmq, the ZeroMQ core engine in C++.

    libzmq is free software; you can redistribute it and/or modify it under
    the terms of the GNU Lesser General Public License (LGPL) as published
    by the Free Software Foundation; either version 3 of the License, or
    (at your option) any later version.

    As a special exception, the Contributors give you permission to link
    this library with independent modules to produce an executable,
    regardless of the license terms of these independent modules, and to
    copy and distribute the resulting executable under terms of your choice,
    provided that you also meet, for each linked independent module, the
    terms and conditions of the license of that module. An independent
    module is a module which is not derived from or based on this library.
    If you modify this library, you must extend this exception to your
    version of the library.

    libzmq is distributed in the hope that it will be useful, but WITHOUT
    ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
    FITNESS FOR A PARTICULAR PURPOSE. See the GNU Lesser General Public
    License for more details.

    You should have received a copy of the GNU Lesser General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

#include "testutil.hpp"
#include <zmq.h>
#include <stdio.h>
#include <stdlib.h>
#include <vector>

void test_system_max ()
{
    // Keep allocating sockets until we run out of system resources
    const int no_of_sockets = 2 * 65536;
    void *ctx = zmq_ctx_new ();
    zmq_ctx_set (ctx, ZMQ_MAX_SOCKETS, no_of_sockets);
    std::vector <void*> sockets;

    while (true) {
        void *socket = zmq_socket (ctx, ZMQ_PAIR);
        if (!socket)
            break;
        sockets.push_back (socket);
    }
    assert ((int) sockets.size () < no_of_sockets);

    //  System is out of resources, further calls to zmq_socket should return NULL
    for (unsigned int i = 0; i < 10; ++i) {
        void *socket = zmq_socket (ctx, ZMQ_PAIR);
        assert (socket == NULL);
    }
    // Clean up.
    for (unsigned int i = 0; i < sockets.size (); ++i)
        zmq_close (sockets [i]);

    zmq_ctx_destroy (ctx);
}

void test_zmq_default_max ()
{
    //  Keep allocating sockets until we hit the default limit
    void *ctx = zmq_ctx_new ();
    std::vector<void*> sockets;

    while (true) {
        void *socket = zmq_socket (ctx, ZMQ_PAIR);
        if (!socket)
            break;
        sockets.push_back (socket);
    }
    //  We may stop sooner if system has fewer available sockets
    assert (sockets.size () <= ZMQ_MAX_SOCKETS_DFLT);

    //  Further calls to zmq_socket should return NULL
    for (unsigned int i = 0; i < 10; ++i) {
        void *socket = zmq_socket (ctx, ZMQ_PAIR);
        assert (socket == NULL);
    }

    //  Clean up
    for (unsigned int i = 0; i < sockets.size (); ++i)
        zmq_close (sockets [i]);

    zmq_ctx_destroy (ctx);
}

int main (void)
{
    setup_test_environment ();

    test_system_max ();
    test_zmq_default_max ();

    return 0;
}
